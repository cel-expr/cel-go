// Code generated from ./Commands.g4 by ANTLR 4.13.1. DO NOT EDIT.

package parser // Commands
import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by CommandsParser.
type CommandsVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by CommandsParser#startCommand.
	VisitStartCommand(ctx *StartCommandContext) any

	// Visit a parse tree produced by CommandsParser#command.
	VisitCommand(ctx *CommandContext) any

	// Visit a parse tree produced by CommandsParser#help.
	VisitHelp(ctx *HelpContext) any

	// Visit a parse tree produced by CommandsParser#let.
	VisitLet(ctx *LetContext) any

	// Visit a parse tree produced by CommandsParser#declare.
	VisitDeclare(ctx *DeclareContext) any

	// Visit a parse tree produced by CommandsParser#varDecl.
	VisitVarDecl(ctx *VarDeclContext) any

	// Visit a parse tree produced by CommandsParser#fnDecl.
	VisitFnDecl(ctx *FnDeclContext) any

	// Visit a parse tree produced by CommandsParser#param.
	VisitParam(ctx *ParamContext) any

	// Visit a parse tree produced by CommandsParser#delete.
	VisitDelete(ctx *DeleteContext) any

	// Visit a parse tree produced by CommandsParser#simple.
	VisitSimple(ctx *SimpleContext) any

	// Visit a parse tree produced by CommandsParser#empty.
	VisitEmpty(ctx *EmptyContext) any

	// Visit a parse tree produced by CommandsParser#compile.
	VisitCompile(ctx *CompileContext) any

	// Visit a parse tree produced by CommandsParser#parse.
	VisitParse(ctx *ParseContext) any

	// Visit a parse tree produced by CommandsParser#exprCmd.
	VisitExprCmd(ctx *ExprCmdContext) any

	// Visit a parse tree produced by CommandsParser#qualId.
	VisitQualId(ctx *QualIdContext) any

	// Visit a parse tree produced by CommandsParser#startType.
	VisitStartType(ctx *StartTypeContext) any

	// Visit a parse tree produced by CommandsParser#type.
	VisitType(ctx *TypeContext) any

	// Visit a parse tree produced by CommandsParser#typeId.
	VisitTypeId(ctx *TypeIdContext) any

	// Visit a parse tree produced by CommandsParser#paramId.
	VisitParamId(ctx *ParamIdContext) any

	// Visit a parse tree produced by CommandsParser#typeParamList.
	VisitTypeParamList(ctx *TypeParamListContext) any

	// Visit a parse tree produced by CommandsParser#start.
	VisitStart(ctx *StartContext) any

	// Visit a parse tree produced by CommandsParser#expr.
	VisitExpr(ctx *ExprContext) any

	// Visit a parse tree produced by CommandsParser#conditionalOr.
	VisitConditionalOr(ctx *ConditionalOrContext) any

	// Visit a parse tree produced by CommandsParser#conditionalAnd.
	VisitConditionalAnd(ctx *ConditionalAndContext) any

	// Visit a parse tree produced by CommandsParser#relation.
	VisitRelation(ctx *RelationContext) any

	// Visit a parse tree produced by CommandsParser#calc.
	VisitCalc(ctx *CalcContext) any

	// Visit a parse tree produced by CommandsParser#MemberExpr.
	VisitMemberExpr(ctx *MemberExprContext) any

	// Visit a parse tree produced by CommandsParser#LogicalNot.
	VisitLogicalNot(ctx *LogicalNotContext) any

	// Visit a parse tree produced by CommandsParser#Negate.
	VisitNegate(ctx *NegateContext) any

	// Visit a parse tree produced by CommandsParser#MemberCall.
	VisitMemberCall(ctx *MemberCallContext) any

	// Visit a parse tree produced by CommandsParser#Select.
	VisitSelect(ctx *SelectContext) any

	// Visit a parse tree produced by CommandsParser#PrimaryExpr.
	VisitPrimaryExpr(ctx *PrimaryExprContext) any

	// Visit a parse tree produced by CommandsParser#Index.
	VisitIndex(ctx *IndexContext) any

	// Visit a parse tree produced by CommandsParser#Ident.
	VisitIdent(ctx *IdentContext) any

	// Visit a parse tree produced by CommandsParser#GlobalCall.
	VisitGlobalCall(ctx *GlobalCallContext) any

	// Visit a parse tree produced by CommandsParser#Nested.
	VisitNested(ctx *NestedContext) any

	// Visit a parse tree produced by CommandsParser#CreateList.
	VisitCreateList(ctx *CreateListContext) any

	// Visit a parse tree produced by CommandsParser#CreateStruct.
	VisitCreateStruct(ctx *CreateStructContext) any

	// Visit a parse tree produced by CommandsParser#CreateMessage.
	VisitCreateMessage(ctx *CreateMessageContext) any

	// Visit a parse tree produced by CommandsParser#ConstantLiteral.
	VisitConstantLiteral(ctx *ConstantLiteralContext) any

	// Visit a parse tree produced by CommandsParser#exprList.
	VisitExprList(ctx *ExprListContext) any

	// Visit a parse tree produced by CommandsParser#listInit.
	VisitListInit(ctx *ListInitContext) any

	// Visit a parse tree produced by CommandsParser#fieldInitializerList.
	VisitFieldInitializerList(ctx *FieldInitializerListContext) any

	// Visit a parse tree produced by CommandsParser#optField.
	VisitOptField(ctx *OptFieldContext) any

	// Visit a parse tree produced by CommandsParser#mapInitializerList.
	VisitMapInitializerList(ctx *MapInitializerListContext) any

	// Visit a parse tree produced by CommandsParser#SimpleIdentifier.
	VisitSimpleIdentifier(ctx *SimpleIdentifierContext) any

	// Visit a parse tree produced by CommandsParser#EscapedIdentifier.
	VisitEscapedIdentifier(ctx *EscapedIdentifierContext) any

	// Visit a parse tree produced by CommandsParser#optExpr.
	VisitOptExpr(ctx *OptExprContext) any

	// Visit a parse tree produced by CommandsParser#Int.
	VisitInt(ctx *IntContext) any

	// Visit a parse tree produced by CommandsParser#Uint.
	VisitUint(ctx *UintContext) any

	// Visit a parse tree produced by CommandsParser#Double.
	VisitDouble(ctx *DoubleContext) any

	// Visit a parse tree produced by CommandsParser#String.
	VisitString(ctx *StringContext) any

	// Visit a parse tree produced by CommandsParser#Bytes.
	VisitBytes(ctx *BytesContext) any

	// Visit a parse tree produced by CommandsParser#BoolTrue.
	VisitBoolTrue(ctx *BoolTrueContext) any

	// Visit a parse tree produced by CommandsParser#BoolFalse.
	VisitBoolFalse(ctx *BoolFalseContext) any

	// Visit a parse tree produced by CommandsParser#Null.
	VisitNull(ctx *NullContext) any
}
