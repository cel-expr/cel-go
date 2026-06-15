// Code generated from /usr/local/google/home/jdtatum/github/cel-go/parser/gen/CEL.g4 by ANTLR 4.13.1. DO NOT EDIT.

package gen // CEL
import "github.com/antlr4-go/antlr/v4"

type BaseCELVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseCELVisitor) VisitStart(ctx *StartContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitExpr(ctx *ExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitConditionalOr(ctx *ConditionalOrContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitConditionalAnd(ctx *ConditionalAndContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitRelation(ctx *RelationContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitCalc(ctx *CalcContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitMemberExpr(ctx *MemberExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitLogicalNot(ctx *LogicalNotContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitNegate(ctx *NegateContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitMemberCall(ctx *MemberCallContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitSelect(ctx *SelectContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitPrimaryExpr(ctx *PrimaryExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitIndex(ctx *IndexContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitIdent(ctx *IdentContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitGlobalCall(ctx *GlobalCallContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitNested(ctx *NestedContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitCreateList(ctx *CreateListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitCreateStruct(ctx *CreateStructContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitCreateMessage(ctx *CreateMessageContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitConstantLiteral(ctx *ConstantLiteralContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitExprList(ctx *ExprListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitListInit(ctx *ListInitContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitFieldInitializerList(ctx *FieldInitializerListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitOptField(ctx *OptFieldContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitMapInitializerList(ctx *MapInitializerListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitSimpleIdentifier(ctx *SimpleIdentifierContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitEscapedIdentifier(ctx *EscapedIdentifierContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitOptExpr(ctx *OptExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitInt(ctx *IntContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitUint(ctx *UintContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitDouble(ctx *DoubleContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitString(ctx *StringContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitBytes(ctx *BytesContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitBoolTrue(ctx *BoolTrueContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitBoolFalse(ctx *BoolFalseContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCELVisitor) VisitNull(ctx *NullContext) any {
	return v.VisitChildren(ctx)
}
