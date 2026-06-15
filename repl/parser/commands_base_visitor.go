// Code generated from ./Commands.g4 by ANTLR 4.13.1. DO NOT EDIT.

package parser // Commands
import "github.com/antlr4-go/antlr/v4"

type BaseCommandsVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseCommandsVisitor) VisitStartCommand(ctx *StartCommandContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitCommand(ctx *CommandContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitHelp(ctx *HelpContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitLet(ctx *LetContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitDeclare(ctx *DeclareContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitVarDecl(ctx *VarDeclContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitFnDecl(ctx *FnDeclContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitParam(ctx *ParamContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitDelete(ctx *DeleteContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitSimple(ctx *SimpleContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitEmpty(ctx *EmptyContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitCompile(ctx *CompileContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitParse(ctx *ParseContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitExprCmd(ctx *ExprCmdContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitQualId(ctx *QualIdContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitStartType(ctx *StartTypeContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitType(ctx *TypeContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitTypeId(ctx *TypeIdContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitParamId(ctx *ParamIdContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitTypeParamList(ctx *TypeParamListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitStart(ctx *StartContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitExpr(ctx *ExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitConditionalOr(ctx *ConditionalOrContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitConditionalAnd(ctx *ConditionalAndContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitRelation(ctx *RelationContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitCalc(ctx *CalcContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitMemberExpr(ctx *MemberExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitLogicalNot(ctx *LogicalNotContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitNegate(ctx *NegateContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitMemberCall(ctx *MemberCallContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitSelect(ctx *SelectContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitPrimaryExpr(ctx *PrimaryExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitIndex(ctx *IndexContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitIdent(ctx *IdentContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitGlobalCall(ctx *GlobalCallContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitNested(ctx *NestedContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitCreateList(ctx *CreateListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitCreateStruct(ctx *CreateStructContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitCreateMessage(ctx *CreateMessageContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitConstantLiteral(ctx *ConstantLiteralContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitExprList(ctx *ExprListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitListInit(ctx *ListInitContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitFieldInitializerList(ctx *FieldInitializerListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitOptField(ctx *OptFieldContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitMapInitializerList(ctx *MapInitializerListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitSimpleIdentifier(ctx *SimpleIdentifierContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitEscapedIdentifier(ctx *EscapedIdentifierContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitOptExpr(ctx *OptExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitInt(ctx *IntContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitUint(ctx *UintContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitDouble(ctx *DoubleContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitString(ctx *StringContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitBytes(ctx *BytesContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitBoolTrue(ctx *BoolTrueContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitBoolFalse(ctx *BoolFalseContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseCommandsVisitor) VisitNull(ctx *NullContext) any {
	return v.VisitChildren(ctx)
}
