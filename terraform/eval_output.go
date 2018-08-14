package terraform

import (
	"log"

	"github.com/hashicorp/hcl2/hcl"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/addrs"
)

// EvalDeleteOutput is an EvalNode implementation that deletes an output
// from the state.
type EvalDeleteOutput struct {
	Addr addrs.OutputValue
}

// TODO: test
func (n *EvalDeleteOutput) Eval(ctx EvalContext) (interface{}, error) {
	state := ctx.State()
	if state == nil {
		return nil, nil
	}

	state.RemoveOutputValue(n.Addr.Absolute(ctx.Path()))
	return nil, nil
}

// EvalWriteOutput is an EvalNode implementation that writes the output
// for the given name to the current state.
type EvalWriteOutput struct {
	Addr      addrs.OutputValue
	Sensitive bool
	Expr      hcl.Expression
	// ContinueOnErr allows interpolation to fail during Input
	ContinueOnErr bool
}

// TODO: test
func (n *EvalWriteOutput) Eval(ctx EvalContext) (interface{}, error) {
	// This has to run before we have a state lock, since evaluation also
	// reads the state
	val, diags := ctx.EvaluateExpr(n.Expr, cty.DynamicPseudoType, nil)
	// We'll handle errors below, after we have loaded the module.

	state := ctx.State()
	if state == nil {
		return nil, nil
	}

	addr := n.Addr.Absolute(ctx.Path())

	// handling the interpolation error
	if diags.HasErrors() {
		if n.ContinueOnErr || flagWarnOutputErrors {
			log.Printf("[ERROR] Output interpolation %q failed: %s", n.Addr.Name, diags.Err())
			// if we're continuing, make sure the output is included, and
			// marked as unknown. If the evaluator was able to find a type
			// for the value in spite of the error then we'll use it.
			state.SetOutputValue(addr, cty.UnknownVal(val.Type()), n.Sensitive)
			return nil, EvalEarlyExitError{}
		}
		return nil, diags.Err()
	}

	state.SetOutputValue(addr, val, n.Sensitive)
	return nil, nil
}
