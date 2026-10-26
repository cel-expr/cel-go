// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repl

import (
	"fmt"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/env"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/repl/parser"

	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// UnparseExprType pretty-prints a type (proto representation) for the REPL.
func UnparseExprType(t *exprpb.Type) string {
	ty, err := cel.ExprTypeToType(t)
	if err != nil {
		return "*unknown type kind*"
	}
	return UnparseType(ty)
}

// UnparseType pretty-prints a type for the REPL.
func UnparseType(t *types.Type) string {
	return env.SerializeTypeDesc(t).SpecifierFormat()
}

type errorListener struct {
	*antlr.DefaultErrorListener
	errs []error
}

func (l *errorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any, line, column int, msg string, e antlr.RecognitionException) {
	l.errs = append(l.errs, fmt.Errorf("type parse error: %s", msg))
	l.DefaultErrorListener.SyntaxError(recognizer, offendingSymbol, line, column, msg, e)
}

type typesVisitor struct {
	parser.BaseCommandsVisitor

	errs []error
}

var _ parser.CommandsVisitor = &typesVisitor{}

type typeParams []*env.TypeDesc

func (t *typesVisitor) Visit(tree antlr.ParseTree) any {
	switch ctx := tree.(type) {
	case *parser.StartTypeContext:
		return t.VisitStartType(ctx)
	case *parser.TypeContext:
		return t.VisitType(ctx)
	case *parser.TypeIdContext:
		return t.VisitTypeID(ctx)
	case *parser.TypeParamListContext:
		return t.VisitTypeParamList(ctx)
	default:
		t.errs = append(t.errs, fmt.Errorf("unhandled parse node kind"))
		return nil
	}

}

func (t *typesVisitor) VisitStartType(ctx *parser.StartTypeContext) any {
	return t.Visit(ctx.GetT())
}

func (t *typesVisitor) expectUnparameterized(p typeParams, id string) {
	if p != nil {
		t.errs = append(t.errs, fmt.Errorf("unexpected type params for %s", id))
	}
}

func checkWellKnown(name string) *env.TypeDesc {
	switch name {
	case "google.protobuf.Timestamp", ".google.protobuf.Timestamp", "timestamp":
		return env.NewTypeDesc("timestamp")
	case "google.protobuf.Duration", ".google.protobuf.Duration", "duration":
		return env.NewTypeDesc("duration")
	case "google.protobuf.Any", ".google.protobuf.Any", "any":
		return env.NewTypeDesc("any")
	case "google.protobuf.Int64Value", ".google.protobuf.Int64Value", "google.protobuf.Int32Value", ".google.protobuf.Int32Value":
		return env.NewTypeDesc("int_wrapper")
	case "google.protobuf.Uint64Value", ".google.protobuf.Uint64Value", "google.protobuf.Uint32Value", ".google.protobuf.Uint32Value":
		return env.NewTypeDesc("uint_wrapper")
	case "google.protobuf.DoubleValue", ".google.protobuf.DoubleValue", "google.protobuf.FloatValue", ".google.protobuf.FloatValue":
		return env.NewTypeDesc("double_wrapper")
	case "google.protobuf.StringValue", ".google.protobuf.StringValue":
		return env.NewTypeDesc("string_wrapper")
	case "google.protobuf.BytesValue", ".google.protobuf.BytesValue":
		return env.NewTypeDesc("bytes_wrapper")
	case "google.protobuf.BoolValue", ".google.protobuf.BoolValue":
		return env.NewTypeDesc("bool_wrapper")
	}
	return nil
}

func (t *typesVisitor) VisitTypeID(ctx *parser.TypeIdContext) any {
	var id strings.Builder
	if ctx.GetLeadingDot() != nil {
		id.WriteString(".")
	}
	tl := ctx.GetId()
	if tl == nil {
		return nil
	}
	id.WriteString(tl.GetText())
	for _, tok := range ctx.GetQualifiers() {
		id.WriteString("." + tok.GetText())
	}
	return id.String()
}

func (t *typesVisitor) VisitTypeParamList(ctx *parser.TypeParamListContext) any {
	var params typeParams
	for _, ty := range ctx.GetTypes() {
		p := t.Visit(ty)
		if p == nil {
			return nil
		}
		params = append(params, p.(*env.TypeDesc))
	}
	return params
}

func (t *typesVisitor) VisitType(ctx *parser.TypeContext) any {

	if ctx.ParamId() != nil {
		param := ctx.ParamId().GetText()
		if !strings.HasPrefix(param, "~") || len(param) < 2 {
			t.errs = append(t.errs, fmt.Errorf("unexpected type param"))
			return nil
		}
		return env.NewTypeParam(strings.TrimPrefix(param, "~"))
	}
	if ctx.TypeId() == nil {
		return nil
	}
	r := t.Visit(ctx.TypeId())
	if r == nil {
		return nil
	}

	typeID := r.(string)

	paramsCtx := ctx.GetParams()

	var params typeParams
	if paramsCtx != nil {
		r = t.Visit(paramsCtx)
		if r == nil {
			return nil
		}
		params = r.(typeParams)
	}

	switch typeID {
	case "int", "uint", "double", "bytes", "string", "bool", "dyn":
		t.expectUnparameterized(params, typeID)
	case "null", "null_type":
		t.expectUnparameterized(params, typeID)
		typeID = "null"
	case "list":
		if params == nil || len(params) != 1 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly one parameter for list"))
			return nil
		}
	case "map":
		if params == nil || len(params) != 2 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly two parameters for map"))
			return nil
		}
	case "type":
		if len(params) > 1 {
			t.errs = append(t.errs, fmt.Errorf("expected 0 or 1 parameter for type"))
			return nil
		}
	case "optional_type":
		if len(params) == 0 {
			params = []*env.TypeDesc{env.NewTypeDesc("dyn")}
		}
		if len(params) != 1 {
			t.errs = append(t.errs, fmt.Errorf("expected exactly one parameter for optional_type"))
			return nil
		}
	default:
		wkt := checkWellKnown(typeID)
		if wkt != nil {
			t.expectUnparameterized(params, typeID)
			return wkt
		}
	}
	return env.NewTypeDesc(typeID, params...)
}

// ParseType parses a human readable type string into the protobuf representation.
func ParseType(t string) (*env.TypeDesc, error) {
	var errListener errorListener
	visitor := &typesVisitor{}
	is := antlr.NewInputStream(t)
	lexer := parser.NewCommandsLexer(is)
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(&errListener)
	p := parser.NewCommandsParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	p.RemoveErrorListeners()
	p.AddErrorListener(&errListener)

	var result *env.TypeDesc
	s := visitor.Visit(p.StartType())
	if s != nil {
		result = s.(*env.TypeDesc)
	}

	errs := append(errListener.errs, visitor.errs...)
	var err error = nil

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		err = fmt.Errorf("errors parsing type:\n%s", strings.Join(msgs, "\n"))
	}

	return result, err
}
