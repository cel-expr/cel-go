// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"cel.dev/cel-go/common"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/runes"
	"cel.dev/cel-go/common/types"
)

type binaryOpInfo struct {
	precedence int
	name       string
	kind       tokenKind
}

var (
	opLogicalOr        = binaryOpInfo{precedence: 1, name: operators.LogicalOr, kind: tokLogicalOr}
	opLogicalAnd       = binaryOpInfo{precedence: 2, name: operators.LogicalAnd, kind: tokLogicalAnd}
	opLess             = binaryOpInfo{precedence: 3, name: operators.Less, kind: tokLess}
	opLessEqual        = binaryOpInfo{precedence: 3, name: operators.LessEquals, kind: tokLessEqual}
	opGreater          = binaryOpInfo{precedence: 3, name: operators.Greater, kind: tokGreater}
	opGreaterEqual     = binaryOpInfo{precedence: 3, name: operators.GreaterEquals, kind: tokGreaterEqual}
	opEqualEqual       = binaryOpInfo{precedence: 3, name: operators.Equals, kind: tokEqualEqual}
	opExclamationEqual = binaryOpInfo{precedence: 3, name: operators.NotEquals, kind: tokExclamationEqual}
	opIn               = binaryOpInfo{precedence: 3, name: operators.In, kind: tokIn}
	opPlus             = binaryOpInfo{precedence: 4, name: operators.Add, kind: tokPlus}
	opMinus            = binaryOpInfo{precedence: 4, name: operators.Subtract, kind: tokMinus}
	opAsterisk         = binaryOpInfo{precedence: 5, name: operators.Multiply, kind: tokAsterisk}
	opSlash            = binaryOpInfo{precedence: 5, name: operators.Divide, kind: tokSlash}
	opPercent          = binaryOpInfo{precedence: 5, name: operators.Modulo, kind: tokPercent}
	opDefault          = binaryOpInfo{precedence: 0, name: "", kind: tokError}
)

func getBinaryOpInfo(kind tokenKind) binaryOpInfo {
	switch kind {
	case tokLogicalOr:
		return opLogicalOr
	case tokLogicalAnd:
		return opLogicalAnd
	case tokLess:
		return opLess
	case tokLessEqual:
		return opLessEqual
	case tokGreater:
		return opGreater
	case tokGreaterEqual:
		return opGreaterEqual
	case tokEqualEqual:
		return opEqualEqual
	case tokExclamationEqual:
		return opExclamationEqual
	case tokIn:
		return opIn
	case tokPlus:
		return opPlus
	case tokMinus:
		return opMinus
	case tokAsterisk:
		return opAsterisk
	case tokSlash:
		return opSlash
	case tokPercent:
		return opPercent
	default:
		return opDefault
	}
}

type prattParserWorker struct {
	content                    runes.Buffer
	length                     int32
	helper                     *parserHelper
	errors                     *parseErrors
	exprFactory                ast.ExprFactory
	lexer                      *lexer
	currTok                    token
	peekTok                    token
	macros                     map[string]Macro
	recursionDepth             int
	recursionLimitExceeded     bool
	errorCount                 int
	maxRecursionDepth          int
	maxExpressionNodeCount     int
	errorReportingLimit        int
	errorRecoveryLimit         int
	populateMacroCalls         bool
	enableOptionalSyntax       bool
	enableVariadicOperatorASTs bool
	enableIdentEscapeSyntax    bool
}

// prattParser encapsulates the context necessary to perform Pratt parsing for different expressions.
type prattParser struct {
	options
}

// Parse parses the expression represented by source using the Pratt parser and returns the result.
func (p *prattParser) Parse(source common.Source) (*ast.AST, *common.Errors) {
	errs := common.NewErrors(source)
	buf, ok := source.(runes.Buffer)
	if !ok {
		buf = runes.NewBuffer(source.Content())
	}
	if buf.Len() > p.expressionSizeCodePointLimit {
		errs.ReportError(common.NoLocation,
			"expression code point size exceeds limit: size: %d, limit %d",
			buf.Len(), p.expressionSizeCodePointLimit)
		return nil, errs
	}
	accu := AccumulatorName
	if p.enableHiddenAccumulatorName {
		accu = HiddenAccumulatorName
	}
	fac := ast.NewExprFactoryWithAccumulator(accu)
	pratt := &prattParserWorker{
		content:                    buf,
		length:                     int32(buf.Len()),
		helper:                     newParserHelper(source, fac),
		errors:                     &parseErrors{errs},
		exprFactory:                fac,
		lexer:                      newLexer(buf),
		macros:                     p.macros,
		maxRecursionDepth:          p.maxRecursionDepth,
		maxExpressionNodeCount:     p.maxExpressionNodeCount,
		errorReportingLimit:        p.errorReportingLimit,
		errorRecoveryLimit:         p.errorRecoveryLimit,
		populateMacroCalls:         p.populateMacroCalls,
		enableOptionalSyntax:       p.enableOptionalSyntax,
		enableVariadicOperatorASTs: p.enableVariadicOperatorASTs,
		enableIdentEscapeSyntax:    p.enableIdentEscapeSyntax,
	}
	pratt.initTokenStream()
	out := pratt.parse()
	if len(errs.GetErrors()) > 0 {
		return nil, errs
	}
	return ast.NewAST(out, pratt.helper.getSourceInfo()), errs
}

func (p *prattParserWorker) initTokenStream() {
	p.currTok = token{kind: tokError, start: 0, end: 0}
	p.peekTok = p.nextSignificantToken(true)
}

func (p *prattParserWorker) isRecoveryLimitExceeded() bool {
	return p.errorCount > p.errorRecoveryLimit
}

func (p *prattParserWorker) nextSignificantToken(reportError bool) token {
	if p.isRecoveryLimitExceeded() {
		return token{kind: tokEnd, start: p.length, end: p.length}
	}
	for {
		tok := p.lexer.Lex()
		if tok.kind == tokWhitespace || tok.kind == tokComment {
			continue
		}
		if tok.kind == tokError && reportError {
			p.reportError(tok, "%s", p.lexer.GetError().message)
			if p.isRecoveryLimitExceeded() {
				return token{kind: tokEnd, start: p.length, end: p.length}
			}
		}
		return tok
	}
}

func (p *prattParserWorker) nextToken() token {
	p.currTok = p.peekTok
	if p.isRecoveryLimitExceeded() {
		p.peekTok = token{kind: tokEnd, start: p.length, end: p.length}
		return p.currTok
	}
	if p.peekTok.kind != tokEnd {
		p.peekTok = p.nextSignificantToken(true)
	}
	return p.currTok
}

func (p *prattParserWorker) tokenText(tok token) string {
	if tok.start >= 0 && tok.end >= tok.start && tok.end <= p.length {
		return p.content.Slice(int(tok.start), int(tok.end))
	}
	return ""
}

func (p *prattParserWorker) nextID(tok token) int64 {
	return p.helper.idFromOffsets(tok.start, tok.end)
}

func (p *prattParserWorker) expect(kind tokenKind, msg string) bool {
	if p.peekTok.kind == kind {
		p.nextToken()
		return true
	}
	if p.isRecoveryLimitExceeded() {
		return false
	}
	if p.peekTok.kind != tokError {
		if msg == "" {
			tokText := p.tokenText(p.peekTok)
			formattedTok := fmt.Sprintf("'%s'", tokText)
			if p.peekTok.kind == tokEnd {
				formattedTok = "<EOF>"
			}
			msg = fmt.Sprintf("Syntax error: mismatched input %s expecting '%s'", formattedTok, kind.String())
		}
		p.reportError(p.peekTok, "%s", msg)
	}
	p.synchronizeOnDelimiter()
	return false
}

func (p *prattParserWorker) synchronizeOnDelimiter() {
	if p.isRecoveryLimitExceeded() {
		p.peekTok = token{kind: tokEnd, start: p.length, end: p.length}
		return
	}
	for p.peekTok.kind != tokEnd {
		if p.peekTok.kind == tokComma ||
			p.peekTok.kind == tokRightParen ||
			p.peekTok.kind == tokRightBracket ||
			p.peekTok.kind == tokRightBrace {
			break
		}
		p.nextToken()
	}
}

func (p *prattParserWorker) reportError(ctx any, format string, args ...any) ast.Expr {
	if p.isRecoveryLimitExceeded() {
		return p.helper.newExpr(common.NoLocation)
	}
	p.errorCount++
	var location common.Location
	err := p.helper.newExpr(ctx)
	switch c := ctx.(type) {
	case common.Location:
		location = c
	case token:
		location = p.helper.getLocation(err.ID())
	default:
		location = p.helper.getLocation(err.ID())
	}
	if p.errorCount <= p.errorReportingLimit {
		p.errors.reportErrorAtID(err.ID(), location, format, args...)
	}
	if p.isRecoveryLimitExceeded() {
		p.peekTok = token{kind: tokEnd, start: p.length, end: p.length}
	}
	return err
}

func (p *prattParserWorker) newLogicManager(function string, term ast.Expr) *logicManager {
	if p.enableVariadicOperatorASTs {
		return newVariadicLogicManager(p.exprFactory, function, term)
	}
	return newBalancingLogicManager(p.exprFactory, function, term)
}

func (p *prattParserWorker) globalCallOrMacro(exprID int64, function string, args ...ast.Expr) ast.Expr {
	if expr, found := p.expandMacro(exprID, function, nil, args...); found {
		return expr
	}
	return p.helper.newGlobalCall(exprID, function, args...)
}

func (p *prattParserWorker) receiverCallOrMacro(exprID int64, function string, target ast.Expr, args ...ast.Expr) ast.Expr {
	if expr, found := p.expandMacro(exprID, function, target, args...); found {
		return expr
	}
	return p.helper.newReceiverCall(exprID, function, target, args...)
}

func (p *prattParserWorker) expandMacro(exprID int64, function string, target ast.Expr, args ...ast.Expr) (ast.Expr, bool) {
	if len(p.macros) == 0 {
		return nil, false
	}
	macro, found := p.macros[makeMacroKey(function, len(args), target != nil)]
	if !found {
		macro, found = p.macros[makeVarArgMacroKey(function, target != nil)]
		if !found {
			return nil, false
		}
	}
	if int(p.helper.expressionCount()) > p.maxExpressionNodeCount {
		loc := p.helper.getLocation(exprID)
		p.helper.deleteID(exprID)
		return p.reportError(loc, "expression count exceeds limit of %d while expanding macro '%s'", p.maxExpressionNodeCount, function), true
	}
	eh := exprHelperPool.Get().(*exprHelper)
	eh.parserHelper = p.helper
	eh.id = exprID
	expr, err := macro.Expander()(eh, target, args)
	exprHelperPool.Put(eh)
	if int(p.helper.expressionCount()) > p.maxExpressionNodeCount {
		loc := p.helper.getLocation(exprID)
		p.helper.deleteID(exprID)
		return p.reportError(loc, "expression count exceeds limit of %d while expanding macro '%s'", p.maxExpressionNodeCount, function), true
	}
	if err != nil {
		loc := err.Location
		if loc == nil {
			loc = p.helper.getLocation(exprID)
		}
		p.helper.deleteID(exprID)
		return p.reportError(loc, "%s", err.Message), true
	}
	if expr == nil {
		return nil, false
	}
	if p.populateMacroCalls {
		p.helper.addMacroCall(expr.ID(), function, target, args...)
	}
	p.helper.deleteID(exprID)
	return expr, true
}

func (p *prattParserWorker) normalizeIdent(tok token, allowQuoted bool) string {
	text := p.tokenText(tok)
	if len(text) == 0 {
		return ""
	}
	if text[0] == '`' {
		if !allowQuoted {
			p.reportError(tok, "unexpected quoted identifier")
			return ""
		}
		if !p.enableIdentEscapeSyntax {
			p.reportError(tok, "unsupported syntax: '`'")
		}
		if len(text) < 2 || text[len(text)-1] != '`' {
			p.reportError(tok, "unterminated quoted identifier")
			return ""
		}
		// Validate the quoted identifier syntax:
		// ESC_IDENTIFIER : '`' (LETTER | DIGIT | '_' | '.' | '-' | '/' | ' ')+ '`';
		inner := text[1 : len(text)-1]
		if len(inner) == 0 {
			p.reportError(tok, "unexpected quoted identifier")
			return ""
		}
		for _, c := range inner {
			if !isAlpha(c) && !isDigit(c) && c != '_' && c != '.' && c != '-' && c != '/' && c != ' ' {
				p.reportError(tok, "unexpected quoted identifier")
				return ""
			}
		}
		return inner
	}
	return text
}

func (p *prattParserWorker) parse() ast.Expr {
	expr := p.parseExpr()
	if p.recursionLimitExceeded || p.isRecoveryLimitExceeded() {
		return expr
	}
	if p.peekTok.kind != tokEnd {
		if p.peekTok.kind != tokError {
			p.reportError(p.peekTok, "Syntax error: mismatched input '%s' expecting <EOF>", p.tokenText(p.peekTok))
		}
		for p.peekTok.kind != tokEnd && !p.isRecoveryLimitExceeded() {
			p.nextToken()
		}
	}
	return expr
}

func (p *prattParserWorker) parseExpr() ast.Expr {
	if p.recursionLimitExceeded || p.isRecoveryLimitExceeded() {
		return p.helper.newExpr(common.NoLocation)
	}
	if p.recursionDepth > p.maxRecursionDepth {
		p.recursionLimitExceeded = true
		p.errors.internalError(fmt.Sprintf("expression recursion limit exceeded: %d", p.maxRecursionDepth))
		return p.helper.newExpr(common.NoLocation)
	}
	p.recursionDepth++
	expr := p.parseBinaryAndTernary(0)
	p.recursionDepth--
	return expr
}

func (p *prattParserWorker) parseBinaryAndTernary(minPrec int) ast.Expr {
	lhs := p.parseSelectorChain()
	for {
		tok := p.peekTok.kind
		if tok == tokQuestion && minPrec <= 0 {
			lhs = p.parseTernary(lhs)
			continue
		}

		opInfo := getBinaryOpInfo(tok)
		if opInfo.kind == tokError || opInfo.precedence < minPrec {
			break
		}

		if opInfo.name == operators.LogicalOr || opInfo.name == operators.LogicalAnd {
			lhs = p.parseLogicalChain(lhs, opInfo)
			continue
		}

		opTok := p.nextToken()
		opID := p.nextID(opTok)
		rhs := p.parseBinaryAndTernary(opInfo.precedence + 1)
		lhs = p.helper.newGlobalCall(opID, opInfo.name, lhs, rhs)
	}
	return lhs
}

func (p *prattParserWorker) parseTernary(lhs ast.Expr) ast.Expr {
	qTok := p.nextToken()
	opID := p.nextID(qTok)
	trueExpr := p.parseBinaryAndTernary(1)
	if !p.expect(tokColon, "expected ':' in conditional expression") {
		return lhs
	}
	falseExpr := p.parseBinaryAndTernary(0)
	return p.helper.newGlobalCall(opID, operators.Conditional, lhs, trueExpr, falseExpr)
}

func (p *prattParserWorker) parseLogicalChain(lhs ast.Expr, opInfo binaryOpInfo) ast.Expr {
	l := p.newLogicManager(opInfo.name, lhs)
	for p.peekTok.kind == opInfo.kind {
		opTok := p.nextToken()
		rhs := p.parseBinaryAndTernary(opInfo.precedence + 1)
		opID := p.nextID(opTok)
		l.addTerm(opID, rhs)
	}
	return l.toExpr()
}

func (p *prattParserWorker) parseSelectorChain() ast.Expr {
	lhs := p.parseUnary()
	return p.parseSelectorChainTail(lhs)
}

func (p *prattParserWorker) parseSelectorChainTail(lhs ast.Expr) ast.Expr {
	for {
		switch p.peekTok.kind {
		case tokDot:
			dotTok := p.nextToken()
			optional := false
			if p.peekTok.kind == tokQuestion {
				p.nextToken()
				optional = true
				if !p.enableOptionalSyntax {
					p.reportError(dotTok, "unsupported syntax '.?'")
				}
			}
			fieldTok := p.nextToken()
			if fieldTok.kind != tokIdent && fieldTok.kind != tokReservedWord {
				if fieldTok.kind != tokError {
					p.reportError(fieldTok, "expected identifier after '.'")
				}
				p.synchronizeOnDelimiter()
				return lhs
			}
			isMemberCall := p.peekTok.kind == tokLeftParen
			field := p.normalizeIdent(fieldTok, !isMemberCall)
			if optional {
				opID := p.nextID(dotTok)
				fieldID := p.nextID(fieldTok)
				lhs = p.helper.newGlobalCall(opID, operators.OptSelect, lhs, p.helper.newLiteralString(fieldID, field))
			} else if isMemberCall {
				lparen := p.nextToken()
				callID := p.nextID(lparen)
				args := p.parseArguments(tokRightParen)
				lhs = p.receiverCallOrMacro(callID, field, lhs, args...)
			} else {
				dotID := p.nextID(dotTok)
				lhs = p.helper.newSelect(dotID, lhs, field)
			}
		case tokLeftBracket:
			bracketTok := p.nextToken()
			opID := p.nextID(bracketTok)
			optional := false
			if p.peekTok.kind == tokQuestion {
				p.nextToken()
				optional = true
				if !p.enableOptionalSyntax {
					p.reportError(bracketTok, "unsupported syntax '?'")
				}
			}
			index := p.parseExpr()
			p.expect(tokRightBracket, "expected ']'")
			opName := operators.Index
			if optional {
				opName = operators.OptIndex
			}
			lhs = p.helper.newGlobalCall(opID, opName, lhs, index)
		case tokLeftBrace:
			if rng, found := p.helper.sourceInfo.GetOffsetRange(lhs.ID()); found {
				if structName, ok := p.extractStructName(lhs); ok {
					objID := p.helper.id(rng)
					lhs = p.parseStruct(objID, structName)
				} else {
					return lhs
				}
			} else {
				return lhs
			}
		default:
			return lhs
		}
	}
}

func (p *prattParserWorker) extractStructName(expr ast.Expr) (string, bool) {
	if expr == nil || expr.Kind() == ast.LiteralKind {
		return "", false
	}
	if expr.Kind() == ast.IdentKind {
		name := expr.AsIdent()
		p.helper.deleteID(expr.ID())
		return name, true
	}
	if expr.Kind() == ast.SelectKind {
		sel := expr.AsSelect()
		if sel.IsTestOnly() {
			return "", false
		}
		prefix, ok := p.extractStructName(sel.Operand())
		if !ok {
			return "", false
		}
		p.helper.deleteID(expr.ID())
		return prefix + "." + sel.FieldName(), true
	}
	return "", false
}

func (p *prattParserWorker) parseStruct(objID int64, structName string) ast.Expr {
	p.nextToken() // consumes {
	var fields []ast.EntryExpr
	for p.peekTok.kind != tokRightBrace && p.peekTok.kind != tokEnd {
		optional := false
		if p.peekTok.kind == tokQuestion {
			q := p.nextToken()
			optional = true
			if !p.enableOptionalSyntax {
				p.reportError(q, "unsupported syntax '?'")
			}
		}
		fieldTok := p.nextToken()
		if fieldTok.kind != tokIdent && fieldTok.kind != tokReservedWord {
			p.reportError(fieldTok, "expected struct field name")
			p.synchronizeOnDelimiter()
			break
		}
		fieldName := p.normalizeIdent(fieldTok, true)
		colonTok := p.peekTok
		if !p.expect(tokColon, "expected ':' in struct field") {
			break
		}
		fieldID := p.nextID(colonTok)
		val := p.parseExpr()
		fields = append(fields, p.helper.newObjectField(fieldID, fieldName, val, optional))
		if p.peekTok.kind == tokComma {
			p.nextToken()
		} else {
			break
		}
	}
	p.expect(tokRightBrace, "expected '}'")
	return p.helper.newObject(objID, structName, fields...)
}

func (p *prattParserWorker) parseUnary() ast.Expr {
	tok := p.peekTok.kind
	if tok == tokExclamation || tok == tokMinus {
		return p.parseUnaryOps()
	}
	return p.parsePrimary()
}

func (p *prattParserWorker) parseUnaryOps() ast.Expr {
	op := p.nextToken()
	if p.peekTok.kind == tokExclamation || p.peekTok.kind == tokMinus {
		return p.parseUnaryOpsChain(op)
	}

	if op.kind == tokMinus {
		if p.peekTok.kind == tokInt {
			return p.parseNegativeIntLiteral(p.nextID(op))
		}
		if p.peekTok.kind == tokFloat {
			return p.parseNegativeDoubleLiteral(p.nextID(op))
		}
	}

	opID := p.nextID(op)
	operand := p.parseSelectorChain()
	opName := operators.LogicalNot
	if op.kind == tokMinus {
		opName = operators.Negate
	}
	return p.globalCallOrMacro(opID, opName, operand)
}

func (p *prattParserWorker) parseUnaryOpsChain(firstOp token) ast.Expr {
	type unaryOpInfo struct {
		kind tokenKind
		id   int64
	}
	ops := []unaryOpInfo{{kind: firstOp.kind, id: p.nextID(firstOp)}}

	for p.peekTok.kind == tokExclamation || p.peekTok.kind == tokMinus {
		op := p.nextToken()
		ops = append(ops, unaryOpInfo{kind: op.kind, id: p.nextID(op)})
	}

	var operand ast.Expr
	if len(ops) > 0 && ops[len(ops)-1].kind == tokMinus {
		switch p.peekTok.kind {
		case tokInt:
			lastOp := ops[len(ops)-1]
			ops = ops[:len(ops)-1]
			operand = p.parseNegativeIntLiteral(lastOp.id)
		case tokFloat:
			lastOp := ops[len(ops)-1]
			ops = ops[:len(ops)-1]
			operand = p.parseNegativeDoubleLiteral(lastOp.id)
		default:
			operand = p.parseSelectorChain()
		}
	} else {
		operand = p.parseSelectorChain()
	}

	for i := len(ops) - 1; i >= 0; i-- {
		opName := operators.LogicalNot
		if ops[i].kind == tokMinus {
			opName = operators.Negate
		}
		operand = p.helper.newGlobalCall(ops[i].id, opName, operand)
	}
	return operand
}

func (p *prattParserWorker) countGroupingParentheses() int {
	if p.peekTok.kind != tokLeftParen {
		return 0
	}
	saved := p.lexer.SavePosition()

	leadingOpenParens := 1
	tok := p.nextSignificantToken(false)
	for tok.kind == tokLeftParen {
		leadingOpenParens++
		tok = p.nextSignificantToken(false)
	}
	if leadingOpenParens == 1 {
		p.lexer.RestorePosition(saved)
		return 1
	}
	openParens := leadingOpenParens
	consecutiveLeadingClosed := 0
	for openParens > 0 {
		if tok.kind == tokEnd || tok.kind == tokError {
			p.lexer.RestorePosition(saved)
			return 1
		}
		switch tok.kind {
		case tokLeftParen:
			openParens++
			consecutiveLeadingClosed = 0
		case tokRightParen:
			if leadingOpenParens == openParens {
				leadingOpenParens--
				consecutiveLeadingClosed++
			} else {
				consecutiveLeadingClosed = 0
			}
			openParens--
		default:
			consecutiveLeadingClosed = 0
		}
		if openParens > 0 {
			tok = p.nextSignificantToken(false)
		}
	}
	p.lexer.RestorePosition(saved)
	if consecutiveLeadingClosed > 1 {
		return consecutiveLeadingClosed
	}
	return 1
}

func (p *prattParserWorker) parsePrimary() ast.Expr {
	switch p.peekTok.kind {
	case tokLeftParen:
		groupingCount := p.countGroupingParentheses()
		for i := 0; i < groupingCount; i++ {
			p.nextToken()
		}
		expr := p.parseExpr()
		for i := 0; i < groupingCount; i++ {
			p.expect(tokRightParen, "expected ')'")
		}
		return expr
	case tokNull:
		return p.helper.exprFactory.NewLiteral(p.nextID(p.nextToken()), types.NullValue)
	case tokTrue:
		tok := p.nextToken()
		return p.helper.newLiteralBool(p.nextID(tok), true)
	case tokFalse:
		tok := p.nextToken()
		return p.helper.newLiteralBool(p.nextID(tok), false)
	case tokInt:
		return p.parseIntLiteral()
	case tokUint:
		return p.parseUintLiteral()
	case tokFloat:
		return p.parseDoubleLiteral()
	case tokString:
		return p.parseStringLiteral()
	case tokBytes:
		return p.parseBytesLiteral()
	case tokLeftBracket:
		return p.parseList()
	case tokLeftBrace:
		return p.parseMap()
	case tokDot, tokIdent, tokReservedWord:
		return p.parseIdentOrCall()
	default:
		badTok := p.nextToken()
		if badTok.kind != tokError {
			if badTok.kind == tokEnd {
				p.reportError(badTok, "Syntax error: mismatched input '<EOF>' expecting expression")
			} else {
				p.reportError(badTok, "unexpected token")
			}
		}
		return p.helper.newExpr(badTok)
	}
}

func (p *prattParserWorker) parseList() ast.Expr {
	openTok := p.nextToken()
	listID := p.nextID(openTok)
	var elems []ast.Expr
	var optionals []int32
	for p.peekTok.kind != tokRightBracket && p.peekTok.kind != tokEnd {
		optional := false
		if p.peekTok.kind == tokQuestion {
			q := p.nextToken()
			optional = true
			if !p.enableOptionalSyntax {
				p.reportError(q, "unsupported syntax '?'")
			}
		}
		if optional {
			optionals = append(optionals, int32(len(elems)))
		}
		elem := p.parseExpr()
		elems = append(elems, elem)
		if p.peekTok.kind == tokComma {
			p.nextToken()
			if p.peekTok.kind == tokRightBracket {
				break
			}
			continue
		}
		break
	}
	p.expect(tokRightBracket, "expected ']'")
	return p.helper.newList(listID, elems, optionals...)
}

func (p *prattParserWorker) parseMap() ast.Expr {
	openTok := p.nextToken()
	mapID := p.nextID(openTok)
	var entries []ast.EntryExpr
	for p.peekTok.kind != tokRightBrace && p.peekTok.kind != tokEnd {
		optional := false
		if p.peekTok.kind == tokQuestion {
			q := p.nextToken()
			optional = true
			if !p.enableOptionalSyntax {
				p.reportError(q, "unsupported syntax '?'")
			}
		}
		key := p.parseExpr()
		colonTok := p.peekTok
		if !p.expect(tokColon, "expected ':' in map entry") {
			break
		}
		entryID := p.nextID(colonTok)
		val := p.parseExpr()
		entries = append(entries, p.helper.newMapEntry(entryID, key, val, optional))
		if p.peekTok.kind == tokComma {
			p.nextToken()
			if p.peekTok.kind == tokRightBrace {
				break
			}
			continue
		}
		break
	}
	p.expect(tokRightBrace, "expected '}'")
	return p.helper.newMap(mapID, entries...)
}

func (p *prattParserWorker) parseIdentOrCall() ast.Expr {
	leadingDot := false
	firstTok := p.peekTok
	if p.peekTok.kind == tokDot {
		p.nextToken()
		leadingDot = true
	}
	idTok := p.nextToken()
	if idTok.kind != tokIdent && idTok.kind != tokReservedWord {
		if idTok.kind != tokError {
			p.reportError(idTok, "expected identifier")
		}
		return p.helper.newExpr(idTok)
	}
	idText := p.normalizeIdent(idTok, false)
	if idTok.kind == tokReservedWord {
		if _, ok := reservedIds[idText]; ok {
			p.reportError(idTok, "reserved identifier: %s", idText)
		}
	}
	name := idText
	if leadingDot {
		name = "." + idText
	}
	id := p.nextID(firstTok)
	if p.peekTok.kind == tokLeftParen {
		p.nextToken()
		args := p.parseArguments(tokRightParen)
		return p.globalCallOrMacro(id, name, args...)
	}
	return p.helper.newIdent(id, name)
}

func (p *prattParserWorker) parseArguments(closeTok tokenKind) []ast.Expr {
	var args []ast.Expr
	if p.peekTok.kind != closeTok && p.peekTok.kind != tokEnd {
		for {
			args = append(args, p.parseExpr())
			if p.peekTok.kind == tokComma {
				p.nextToken()
				if p.peekTok.kind == closeTok {
					p.reportError(p.peekTok, "unexpected token")
					break
				}
				continue
			}
			break
		}
	}
	p.expect(closeTok, "")
	return args
}

func (p *prattParserWorker) parseIntLiteral() ast.Expr {
	tok := p.nextToken()
	id := p.nextID(tok)
	text := p.tokenText(tok)
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
		text = text[2:]
	}
	val, err := strconv.ParseInt(text, base, 64)
	if err != nil {
		return p.reportError(tok, "invalid int literal")
	}
	return p.helper.newLiteralInt(id, val)
}

func (p *prattParserWorker) parseNegativeIntLiteral(opID int64) ast.Expr {
	tok := p.nextToken()
	text := p.tokenText(tok)
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
		text = text[2:]
	}
	val, err := strconv.ParseInt("-"+text, base, 64)
	if err != nil {
		return p.reportError(tok, "invalid int literal")
	}
	return p.helper.newLiteralInt(opID, val)
}

func (p *prattParserWorker) parseUintLiteral() ast.Expr {
	tok := p.nextToken()
	id := p.nextID(tok)
	text := p.tokenText(tok)
	text = text[:len(text)-1]
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base = 16
		text = text[2:]
	}
	val, err := strconv.ParseUint(text, base, 64)
	if err != nil {
		return p.reportError(tok, "invalid uint literal")
	}
	return p.helper.newLiteralUint(id, val)
}

func (p *prattParserWorker) parseDoubleLiteral() ast.Expr {
	tok := p.nextToken()
	id := p.nextID(tok)
	text := p.tokenText(tok)
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return p.reportError(tok, "invalid double literal")
	}
	return p.helper.newLiteralDouble(id, val)
}

func (p *prattParserWorker) parseNegativeDoubleLiteral(opID int64) ast.Expr {
	tok := p.nextToken()
	text := p.tokenText(tok)
	val, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return p.reportError(tok, "invalid double literal")
	}
	return p.helper.newLiteralDouble(opID, -val)
}

func (p *prattParserWorker) parseStringLiteral() ast.Expr {
	tok := p.nextToken()
	id := p.nextID(tok)
	text := p.tokenText(tok)
	unescaped, err := unescape(text, false)
	if err != nil {
		return p.reportError(tok, "%s", err.Error())
	}
	return p.helper.newLiteralString(id, unescaped)
}

func (p *prattParserWorker) parseBytesLiteral() ast.Expr {
	tok := p.nextToken()
	id := p.nextID(tok)
	text := p.tokenText(tok)
	if strings.HasPrefix(text, "b") || strings.HasPrefix(text, "B") {
		text = text[1:]
	} else if strings.HasPrefix(text, "rb") || strings.HasPrefix(text, "RB") || strings.HasPrefix(text, "rB") || strings.HasPrefix(text, "Rb") {
		text = "r" + text[2:]
	} else if strings.HasPrefix(text, "br") || strings.HasPrefix(text, "BR") || strings.HasPrefix(text, "bR") || strings.HasPrefix(text, "Br") {
		text = text[1:]
	}
	unescaped, err := unescape(text, true)
	if err != nil {
		return p.reportError(tok, "%s", err.Error())
	}
	return p.helper.newLiteralBytes(id, []byte(unescaped))
}
