// Code generated from /usr/local/google/home/jdtatum/github/cel-go/parser/gen/CEL.g4 by ANTLR 4.13.1. DO NOT EDIT.

package gen // CEL
import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by CELParser.
type CELVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by CELParser#start.
	VisitStart(ctx *StartContext) any

	// Visit a parse tree produced by CELParser#expr.
	VisitExpr(ctx *ExprContext) any

	// Visit a parse tree produced by CELParser#conditionalOr.
	VisitConditionalOr(ctx *ConditionalOrContext) any

	// Visit a parse tree produced by CELParser#conditionalAnd.
	VisitConditionalAnd(ctx *ConditionalAndContext) any

	// Visit a parse tree produced by CELParser#relation.
	VisitRelation(ctx *RelationContext) any

	// Visit a parse tree produced by CELParser#calc.
	VisitCalc(ctx *CalcContext) any

	// Visit a parse tree produced by CELParser#MemberExpr.
	VisitMemberExpr(ctx *MemberExprContext) any

	// Visit a parse tree produced by CELParser#LogicalNot.
	VisitLogicalNot(ctx *LogicalNotContext) any

	// Visit a parse tree produced by CELParser#Negate.
	VisitNegate(ctx *NegateContext) any

	// Visit a parse tree produced by CELParser#MemberCall.
	VisitMemberCall(ctx *MemberCallContext) any

	// Visit a parse tree produced by CELParser#Select.
	VisitSelect(ctx *SelectContext) any

	// Visit a parse tree produced by CELParser#PrimaryExpr.
	VisitPrimaryExpr(ctx *PrimaryExprContext) any

	// Visit a parse tree produced by CELParser#Index.
	VisitIndex(ctx *IndexContext) any

	// Visit a parse tree produced by CELParser#Ident.
	VisitIdent(ctx *IdentContext) any

	// Visit a parse tree produced by CELParser#GlobalCall.
	VisitGlobalCall(ctx *GlobalCallContext) any

	// Visit a parse tree produced by CELParser#Nested.
	VisitNested(ctx *NestedContext) any

	// Visit a parse tree produced by CELParser#CreateList.
	VisitCreateList(ctx *CreateListContext) any

	// Visit a parse tree produced by CELParser#CreateStruct.
	VisitCreateStruct(ctx *CreateStructContext) any

	// Visit a parse tree produced by CELParser#CreateMessage.
	VisitCreateMessage(ctx *CreateMessageContext) any

	// Visit a parse tree produced by CELParser#ConstantLiteral.
	VisitConstantLiteral(ctx *ConstantLiteralContext) any

	// Visit a parse tree produced by CELParser#exprList.
	VisitExprList(ctx *ExprListContext) any

	// Visit a parse tree produced by CELParser#listInit.
	VisitListInit(ctx *ListInitContext) any

	// Visit a parse tree produced by CELParser#fieldInitializerList.
	VisitFieldInitializerList(ctx *FieldInitializerListContext) any

	// Visit a parse tree produced by CELParser#optField.
	VisitOptField(ctx *OptFieldContext) any

	// Visit a parse tree produced by CELParser#mapInitializerList.
	VisitMapInitializerList(ctx *MapInitializerListContext) any

	// Visit a parse tree produced by CELParser#SimpleIdentifier.
	VisitSimpleIdentifier(ctx *SimpleIdentifierContext) any

	// Visit a parse tree produced by CELParser#EscapedIdentifier.
	VisitEscapedIdentifier(ctx *EscapedIdentifierContext) any

	// Visit a parse tree produced by CELParser#optExpr.
	VisitOptExpr(ctx *OptExprContext) any

	// Visit a parse tree produced by CELParser#Int.
	VisitInt(ctx *IntContext) any

	// Visit a parse tree produced by CELParser#Uint.
	VisitUint(ctx *UintContext) any

	// Visit a parse tree produced by CELParser#Double.
	VisitDouble(ctx *DoubleContext) any

	// Visit a parse tree produced by CELParser#String.
	VisitString(ctx *StringContext) any

	// Visit a parse tree produced by CELParser#Bytes.
	VisitBytes(ctx *BytesContext) any

	// Visit a parse tree produced by CELParser#BoolTrue.
	VisitBoolTrue(ctx *BoolTrueContext) any

	// Visit a parse tree produced by CELParser#BoolFalse.
	VisitBoolFalse(ctx *BoolFalseContext) any

	// Visit a parse tree produced by CELParser#Null.
	VisitNull(ctx *NullContext) any
}
