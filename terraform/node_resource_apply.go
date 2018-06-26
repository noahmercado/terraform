package terraform

import (
	"fmt"

	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/configs"
	"github.com/zclconf/go-cty/cty"
)

// NodeApplyableResourceInstance represents a resource that is "applyable":
// it is ready to be applied and is represented by a diff.
type NodeApplyableResourceInstance struct {
	*NodeAbstractResourceInstance

	destroyNode GraphNodeDestroyerCBD
}

var (
	_ GraphNodeResource         = (*NodeApplyableResourceInstance)(nil)
	_ GraphNodeResourceInstance = (*NodeApplyableResourceInstance)(nil)
	_ GraphNodeCreator          = (*NodeApplyableResourceInstance)(nil)
	_ GraphNodeReferencer       = (*NodeApplyableResourceInstance)(nil)
	_ GraphNodeEvalable         = (*NodeApplyableResourceInstance)(nil)
)

// GraphNodeAttachDestroyer
func (n *NodeApplyableResourceInstance) AttachDestroyNode(d GraphNodeDestroyerCBD) {
	n.destroyNode = d
}

// createBeforeDestroy checks this nodes config status and the status af any
// companion destroy node for CreateBeforeDestroy.
func (n *NodeApplyableResourceInstance) createBeforeDestroy() bool {
	cbd := false

	if n.Config != nil && n.Config.Managed != nil {
		cbd = n.Config.Managed.CreateBeforeDestroy
	}

	if n.destroyNode != nil {
		cbd = cbd || n.destroyNode.CreateBeforeDestroy()
	}

	return cbd
}

// GraphNodeCreator
func (n *NodeApplyableResourceInstance) CreateAddr() *addrs.AbsResourceInstance {
	addr := n.ResourceInstanceAddr()
	return &addr
}

// GraphNodeReferencer, overriding NodeAbstractResourceInstance
func (n *NodeApplyableResourceInstance) References() []*addrs.Reference {
	// Start with the usual resource instance implementation
	ret := n.NodeAbstractResourceInstance.References()

	// Applying a resource must also depend on the destruction of any of its
	// dependencies, since this may for example affect the outcome of
	// evaluating an entire list of resources with "count" set (by reducing
	// the count).
	//
	// However, we can't do this in create_before_destroy mode because that
	// would create a dependency cycle. We make a compromise here of requiring
	// changes to be updated across two applies in this case, since the first
	// plan will use the old values.
	if !n.createBeforeDestroy() {
		for _, ref := range ret {
			switch tr := ref.Subject.(type) {
			case addrs.ResourceInstance:
				newRef := *ref // shallow copy so we can mutate
				newRef.Subject = tr.Phase(addrs.ResourceInstancePhaseDestroy)
				newRef.Remaining = nil // can't access attributes of something being destroyed
				ret = append(ret, &newRef)
			case addrs.Resource:
				newRef := *ref // shallow copy so we can mutate
				newRef.Subject = tr.Phase(addrs.ResourceInstancePhaseDestroy)
				newRef.Remaining = nil // can't access attributes of something being destroyed
				ret = append(ret, &newRef)
			}
		}
	}

	return ret
}

// GraphNodeEvalable
func (n *NodeApplyableResourceInstance) EvalTree() EvalNode {
	addr := n.ResourceInstanceAddr()

	// State still uses legacy-style internal ids, so we need to shim to get
	// a suitable key to use.
	stateId := NewLegacyResourceInstanceAddress(addr).stateId()

	// Determine the dependencies for the state.
	stateDeps := n.StateReferences()
	// filter out self-references
	filtered := []string{}
	for _, d := range stateDeps {
		if d != dottedInstanceAddr(addr.Resource) {
			filtered = append(filtered, d)
		}
	}
	stateDeps = filtered

	// Eval info is different depending on what kind of resource this is
	switch n.Config.Mode {
	case addrs.ManagedResourceMode:
		return n.evalTreeManagedResource(addr, stateId, stateDeps)
	case addrs.DataResourceMode:
		return n.evalTreeDataResource(addr, stateId, stateDeps)
	default:
		panic(fmt.Errorf("unsupported resource mode %s", n.Config.Mode))
	}
}

func (n *NodeApplyableResourceInstance) evalTreeDataResource(addr addrs.AbsResourceInstance, stateId string, stateDeps []string) EvalNode {
	var provider ResourceProvider
	var providerSchema *ProviderSchema
	var diff *InstanceDiff
	var state *InstanceState
	var configVal cty.Value

	return &EvalSequence{
		Nodes: []EvalNode{
			// Get the saved diff for apply
			&EvalReadDiff{
				Name: stateId,
				Diff: &diff,
			},

			// Stop early if we don't actually have a diff
			&EvalIf{
				If: func(ctx EvalContext) (bool, error) {
					if diff == nil {
						return true, EvalEarlyExitError{}
					}

					if diff.GetAttributesLen() == 0 {
						return true, EvalEarlyExitError{}
					}

					return true, nil
				},
				Then: EvalNoop{},
			},

			&EvalGetProvider{
				Addr:   n.ResolvedProvider,
				Output: &provider,
				Schema: &providerSchema,
			},

			// Make a new diff, in case we've learned new values in the state
			// during apply which we can now incorporate.
			&EvalReadDataDiff{
				Addr:           addr.Resource,
				Config:         n.Config,
				Provider:       &provider,
				ProviderSchema: &providerSchema,
				Output:         &diff,
				OutputValue:    &configVal,
				OutputState:    &state,
			},

			&EvalReadDataApply{
				Addr:     addr.Resource,
				Diff:     &diff,
				Provider: &provider,
				Output:   &state,
			},

			&EvalWriteState{
				Name:         stateId,
				ResourceType: n.Config.Type,
				Provider:     n.ResolvedProvider,
				Dependencies: stateDeps,
				State:        &state,
			},

			// Clear the diff now that we've applied it, so
			// later nodes won't see a diff that's now a no-op.
			&EvalWriteDiff{
				Name: stateId,
				Diff: nil,
			},

			&EvalUpdateStateHook{},
		},
	}
}

func (n *NodeApplyableResourceInstance) evalTreeManagedResource(addr addrs.AbsResourceInstance, stateId string, stateDeps []string) EvalNode {
	// Declare a bunch of variables that are used for state during
	// evaluation. Most of this are written to by-address below.
	var provider ResourceProvider
	var providerSchema *ProviderSchema
	var diff, diffApply *InstanceDiff
	var state *InstanceState
	var err error
	var createNew bool
	var createBeforeDestroyEnabled bool
	var configVal cty.Value

	return &EvalSequence{
		Nodes: []EvalNode{
			// Get the saved diff for apply
			&EvalReadDiff{
				Name: stateId,
				Diff: &diffApply,
			},

			// We don't want to do any destroys
			// (these are handled by NodeDestroyResourceInstance instead)
			&EvalIf{
				If: func(ctx EvalContext) (bool, error) {
					if diffApply == nil {
						return true, EvalEarlyExitError{}
					}

					if diffApply.GetDestroy() && diffApply.GetAttributesLen() == 0 {
						return true, EvalEarlyExitError{}
					}

					diffApply.SetDestroy(false)
					return true, nil
				},
				Then: EvalNoop{},
			},

			&EvalIf{
				If: func(ctx EvalContext) (bool, error) {
					destroy := false
					if diffApply != nil {
						destroy = diffApply.GetDestroy() || diffApply.RequiresNew()
					}

					if destroy && n.createBeforeDestroy() {
						createBeforeDestroyEnabled = true
					}

					return createBeforeDestroyEnabled, nil
				},
				Then: &EvalDeposeState{
					Name: stateId,
				},
			},

			&EvalGetProvider{
				Addr:   n.ResolvedProvider,
				Output: &provider,
				Schema: &providerSchema,
			},
			&EvalReadState{
				Name:   stateId,
				Output: &state,
			},

			// Make a new diff, in case we've learned new values in the state
			// during apply which we can now incorporate.
			&EvalDiff{
				Addr:           addr.Resource,
				Config:         n.Config,
				Provider:       &provider,
				ProviderSchema: &providerSchema,
				State:          &state,
				OutputDiff:     &diffApply,
				OutputValue:    &configVal,
				OutputState:    &state,
			},

			// Get the saved diff
			&EvalReadDiff{
				Name: stateId,
				Diff: &diff,
			},

			// Compare the diffs
			&EvalCompareDiff{
				Addr: addr.Resource,
				One:  &diff,
				Two:  &diffApply,
			},

			&EvalGetProvider{
				Addr:   n.ResolvedProvider,
				Output: &provider,
				Schema: &providerSchema,
			},
			&EvalReadState{
				Name:   stateId,
				Output: &state,
			},

			// Call pre-apply hook
			&EvalApplyPre{
				Addr:  addr.Resource,
				State: &state,
				Diff:  &diffApply,
			},
			&EvalApply{
				Addr:      addr.Resource,
				State:     &state,
				Diff:      &diffApply,
				Provider:  &provider,
				Output:    &state,
				Error:     &err,
				CreateNew: &createNew,
			},
			&EvalWriteState{
				Name:         stateId,
				ResourceType: n.Config.Type,
				Provider:     n.ResolvedProvider,
				Dependencies: stateDeps,
				State:        &state,
			},
			&EvalApplyProvisioners{
				Addr:           addr.Resource,
				State:          &state,
				ResourceConfig: n.Config,
				CreateNew:      &createNew,
				Error:          &err,
				When:           configs.ProvisionerWhenCreate,
			},
			&EvalIf{
				If: func(ctx EvalContext) (bool, error) {
					return createBeforeDestroyEnabled && err != nil, nil
				},
				Then: &EvalUndeposeState{
					Name:  stateId,
					State: &state,
				},
				Else: &EvalWriteState{
					Name:         stateId,
					ResourceType: n.Config.Type,
					Provider:     n.ResolvedProvider,
					Dependencies: stateDeps,
					State:        &state,
				},
			},

			// We clear the diff out here so that future nodes
			// don't see a diff that is already complete. There
			// is no longer a diff!
			&EvalWriteDiff{
				Name: stateId,
				Diff: nil,
			},

			&EvalApplyPost{
				Addr:  addr.Resource,
				State: &state,
				Error: &err,
			},
			&EvalUpdateStateHook{},
		},
	}
}
