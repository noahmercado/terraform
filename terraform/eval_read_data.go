package terraform

import (
	"fmt"

	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/states"

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/addrs"
	"github.com/hashicorp/terraform/configs"
	"github.com/hashicorp/terraform/plans"
)

// EvalReadDataDiff is an EvalNode implementation that executes a data
// resource's ReadDataDiff method to discover what attributes it exports.
type EvalReadDataDiff struct {
	Addr           addrs.ResourceInstance
	Config         *configs.Resource
	Provider       *providers.Interface
	ProviderSchema **ProviderSchema

	Output      **plans.ResourceInstanceChange
	OutputValue *cty.Value
	OutputState **states.ResourceInstanceObject

	// Set Previous when re-evaluating diff during apply, to ensure that
	// the "Destroy" flag is preserved.
	Previous **plans.ResourceInstanceChange
}

func (n *EvalReadDataDiff) Eval(ctx EvalContext) (interface{}, error) {
	return nil, fmt.Errorf("EvalReadDataDiff not yet updated for new state/plan/provider types")
	/*
		absAddr := n.Addr.Absolute(ctx.Path())

		if n.ProviderSchema == nil || *n.ProviderSchema == nil {
			return nil, fmt.Errorf("provider schema not available for %s", n.Addr)
		}

		var diags tfdiags.Diagnostics

		// The provider API still expects our legacy InstanceInfo type.
		legacyInfo := NewInstanceInfo(n.Addr.Absolute(ctx.Path()))

		err := ctx.Hook(func(h Hook) (HookAction, error) {
			return h.PreDiff(absAddr, cty.NullVal(cty.DynamicPseudoType), cty.NullVal(cty.DynamicPseudoType))
		})
		if err != nil {
			return nil, err
		}

		var diff *InstanceDiff
		var configVal cty.Value

		if n.Previous != nil && *n.Previous != nil && (*n.Previous).GetDestroy() {
			// If we're re-diffing for a diff that was already planning to
			// destroy, then we'll just continue with that plan.
			diff = &InstanceDiff{Destroy: true}
		} else {
			provider := *n.Provider
			config := *n.Config
			providerSchema := *n.ProviderSchema
			schema := providerSchema.DataSources[n.Addr.Resource.Type]
			if schema == nil {
				// Should be caught during validation, so we don't bother with a pretty error here
				return nil, fmt.Errorf("provider does not support data source %q", n.Addr.Resource.Type)
			}

			keyData := EvalDataForInstanceKey(n.Addr.Key)

			var configDiags tfdiags.Diagnostics
			configVal, _, configDiags = ctx.EvaluateBlock(config.Config, schema, nil, keyData)
			diags = diags.Append(configDiags)
			if configDiags.HasErrors() {
				return nil, diags.Err()
			}

			// The provider API still expects our legacy ResourceConfig type.
			legacyRC := NewResourceConfigShimmed(configVal, schema)

			var err error
			diff, err = provider.ReadDataDiff(legacyInfo, legacyRC)
			if err != nil {
				diags = diags.Append(err)
				return nil, diags.Err()
			}
			if diff == nil {
				diff = new(InstanceDiff)
			}

			// if id isn't explicitly set then it's always computed, because we're
			// always "creating a new resource".
			diff.init()
			if _, ok := diff.Attributes["id"]; !ok {
				diff.SetAttribute("id", &ResourceAttrDiff{
					Old:         "",
					NewComputed: true,
					RequiresNew: true,
					Type:        DiffAttrOutput,
				})
			}
		}

		err = ctx.Hook(func(h Hook) (HookAction, error) {
			return h.PostDiff(legacyInfo, diff)
		})
		if err != nil {
			return nil, err
		}

		*n.Output = diff

		if n.OutputValue != nil {
			*n.OutputValue = configVal
		}

		if n.OutputState != nil {
			state := &InstanceState{}
			*n.OutputState = state

			// Apply the diff to the returned state, so the state includes
			// any attribute values that are not computed.
			if !diff.Empty() && n.OutputState != nil {
				*n.OutputState = state.MergeDiff(diff)
			}
		}

		return nil, diags.ErrWithWarnings()
	*/
}

// EvalReadDataApply is an EvalNode implementation that executes a data
// resource's ReadDataApply method to read data from the data source.
type EvalReadDataApply struct {
	Addr     addrs.ResourceInstance
	Provider *providers.Interface
	Output   **states.ResourceInstanceObject
	Change   **plans.ResourceInstanceChange
}

func (n *EvalReadDataApply) Eval(ctx EvalContext) (interface{}, error) {
	return nil, fmt.Errorf("EvalReadDataApply not yet updated for new state/plan/provider types")
	/*
		provider := *n.Provider
		change := *n.Change
		absAddr := n.Addr.Absolute(ctx.Path())

		// The provider and hook APIs still expect our legacy InstanceInfo type.
		legacyInfo := NewInstanceInfo(n.Addr.Absolute(ctx.Path()))

		// If the diff is for *destroying* this resource then we'll
		// just drop its state and move on, since data resources don't
		// support an actual "destroy" action.
		if diff != nil && diff.GetDestroy() {
			if n.Output != nil {
				*n.Output = nil
			}
			return nil, nil
		}

		// For the purpose of external hooks we present a data apply as a
		// "Refresh" rather than an "Apply" because creating a data source
		// is presented to users/callers as a "read" operation.
		err := ctx.Hook(func(h Hook) (HookAction, error) {
			// We don't have a state yet, so we'll just give the hook an
			// empty one to work with.
			return h.PreRefresh(absAddr, cty.NullVal(cty.DynamicPseudoType))
		})
		if err != nil {
			return nil, err
		}

		state, err := provider.ReadDataApply(legacyInfo, diff)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", n.Addr.Absolute(ctx.Path()).String(), err)
		}

		err = ctx.Hook(func(h Hook) (HookAction, error) {
			return h.PostRefresh(absAddr, state)
		})
		if err != nil {
			return nil, err
		}

		if n.Output != nil {
			*n.Output = state
		}

		return nil, nil
	*/
}
