package lang

import (
	"fmt"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"

	"github.com/hashicorp/terraform/lang/funcs"
)

var impureFunctions = []string{
	"bcrypt",
	"timestamp",
	"uuid",
}

// Functions returns the set of functions that should be used to when evaluating
// expressions in the receiving scope.
func (s *Scope) Functions() map[string]function.Function {
	s.funcsLock.Lock()
	if s.funcs == nil {
		// Some of our functions are just directly the cty stdlib functions.
		// Others are implemented in the subdirectory "funcs" here in this
		// repository. New functions should generally start out their lives
		// in the "funcs" directory and potentially graduate to cty stdlib
		// later if the functionality seems to be something domain-agnostic
		// that would be useful to all applications using cty functions.

		s.funcs = map[string]function.Function{
			"abs":          stdlib.AbsoluteFunc,
			"basename":     funcs.BasenameFunc,
			"base64decode": funcs.Base64DecodeFunc,
			"base64encode": funcs.Base64EncodeFunc,
			"base64gzip":   funcs.Base64GzipFunc,
			"base64sha256": funcs.Base64Sha256Func,
			"base64sha512": funcs.Base64Sha512Func,
			"bcrypt":       funcs.BcryptFunc,
			"ceil":         funcs.CeilFunc,
			"chomp":        unimplFunc, // TODO
			"cidrhost":     unimplFunc, // TODO
			"cidrnetmask":  unimplFunc, // TODO
			"cidrsubnet":   unimplFunc, // TODO
			"coalesce":     stdlib.CoalesceFunc,
			"coalescelist": unimplFunc, // TODO
			"compact":      unimplFunc, // TODO
			"concat":       stdlib.ConcatFunc,
			"contains":     unimplFunc, // TODO
			"csvdecode":    stdlib.CSVDecodeFunc,
			"dirname":      funcs.DirnameFunc,
			"distinct":     unimplFunc, // TODO
			"element":      funcs.ElementFunc,
			"chunklist":    unimplFunc, // TODO
			"file":         funcs.MakeFileFunc(s.BaseDir, false),
			"filebase64":   funcs.MakeFileFunc(s.BaseDir, true),
			"matchkeys":    unimplFunc, // TODO
			"flatten":      unimplFunc, // TODO
			"floor":        unimplFunc, // TODO
			"format":       stdlib.FormatFunc,
			"formatlist":   stdlib.FormatListFunc,
			"indent":       unimplFunc, // TODO
			"index":        unimplFunc, // TODO
			"join":         funcs.JoinFunc,
			"jsondecode":   stdlib.JSONDecodeFunc,
			"jsonencode":   stdlib.JSONEncodeFunc,
			"keys":         unimplFunc, // TODO
			"length":       funcs.LengthFunc,
			"list":         unimplFunc, // TODO
			"log":          unimplFunc, // TODO
			"lookup":       unimplFunc, // TODO
			"lower":        stdlib.LowerFunc,
			"map":          unimplFunc, // TODO
			"max":          stdlib.MaxFunc,
			"md5":          funcs.Md5Func,
			"merge":        unimplFunc, // TODO
			"min":          stdlib.MinFunc,
			"pathexpand":   funcs.PathExpandFunc,
			"pow":          unimplFunc, // TODO
			"replace":      unimplFunc, // TODO
			"rsadecrypt":   funcs.RsaDecryptFunc,
			"sha1":         funcs.Sha1Func,
			"sha256":       funcs.Sha256Func,
			"sha512":       funcs.Sha512Func,
			"signum":       unimplFunc, // TODO
			"slice":        unimplFunc, // TODO
			"sort":         funcs.SortFunc,
			"split":        funcs.SplitFunc,
			"substr":       stdlib.SubstrFunc,
			"timestamp":    funcs.TimestampFunc,
			"timeadd":      funcs.TimeAddFunc,
			"title":        unimplFunc, // TODO
			"transpose":    unimplFunc, // TODO
			"trimspace":    unimplFunc, // TODO
			"upper":        stdlib.UpperFunc,
			"urlencode":    funcs.URLEncodeFunc,
			"uuid":         funcs.UUIDFunc,
			"values":       unimplFunc, // TODO
			"zipmap":       unimplFunc, // TODO
		}

		if s.PureOnly {
			// Force our few impure functions to return unknown so that we
			// can defer evaluating them until a later pass.
			for _, name := range impureFunctions {
				s.funcs[name] = function.Unpredictable(s.funcs[name])
			}
		}
	}
	s.funcsLock.Unlock()

	return s.funcs
}

var unimplFunc = function.New(&function.Spec{
	Type: func([]cty.Value) (cty.Type, error) {
		return cty.DynamicPseudoType, fmt.Errorf("function not yet implemented")
	},
	Impl: func([]cty.Value, cty.Type) (cty.Value, error) {
		return cty.DynamicVal, fmt.Errorf("function not yet implemented")
	},
})
