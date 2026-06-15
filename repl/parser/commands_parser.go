// Code generated from ./Commands.g4 by ANTLR 4.13.1. DO NOT EDIT.

package parser // Commands
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr4-go/antlr/v4"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type CommandsParser struct {
	*antlr.BaseParser
}

var CommandsParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func commandsParserInit() {
	staticData := &CommandsParserStaticData
	staticData.LiteralNames = []string{
		"", "'%help'", "'%?'", "'%let'", "'%declare'", "'%delete'", "'%compile'",
		"'%parse'", "'%eval'", "'--'", "", "", "'->'", "'='", "", "'=='", "'!='",
		"'in'", "'<'", "'<='", "'>='", "'>'", "'&&'", "'||'", "'['", "']'",
		"'{'", "'}'", "'('", "')'", "'.'", "','", "'-'", "'!'", "'?'", "':'",
		"'+'", "'*'", "'/'", "'%'", "'true'", "'false'", "'null'",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "", "", "", "COMMAND", "FLAG", "ARROW",
		"EQUAL_ASSIGN", "PARAM_SPECIFIER", "EQUALS", "NOT_EQUALS", "IN", "LESS",
		"LESS_EQUALS", "GREATER_EQUALS", "GREATER", "LOGICAL_AND", "LOGICAL_OR",
		"LBRACKET", "RPRACKET", "LBRACE", "RBRACE", "LPAREN", "RPAREN", "DOT",
		"COMMA", "MINUS", "EXCLAM", "QUESTIONMARK", "COLON", "PLUS", "STAR",
		"SLASH", "PERCENT", "CEL_TRUE", "CEL_FALSE", "NUL", "WHITESPACE", "COMMENT",
		"NUM_FLOAT", "NUM_INT", "NUM_UINT", "STRING", "BYTES", "IDENTIFIER",
		"ESC_IDENTIFIER",
	}
	staticData.RuleNames = []string{
		"startCommand", "command", "help", "let", "declare", "varDecl", "fnDecl",
		"param", "delete", "simple", "empty", "compile", "parse", "exprCmd",
		"qualId", "startType", "type", "typeId", "paramId", "typeParamList",
		"start", "expr", "conditionalOr", "conditionalAnd", "relation", "calc",
		"unary", "member", "primary", "exprList", "listInit", "fieldInitializerList",
		"optField", "mapInitializerList", "escapeIdent", "optExpr", "literal",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 51, 439, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 2, 8, 7, 8, 2, 9, 7, 9, 2, 10, 7,
		10, 2, 11, 7, 11, 2, 12, 7, 12, 2, 13, 7, 13, 2, 14, 7, 14, 2, 15, 7, 15,
		2, 16, 7, 16, 2, 17, 7, 17, 2, 18, 7, 18, 2, 19, 7, 19, 2, 20, 7, 20, 2,
		21, 7, 21, 2, 22, 7, 22, 2, 23, 7, 23, 2, 24, 7, 24, 2, 25, 7, 25, 2, 26,
		7, 26, 2, 27, 7, 27, 2, 28, 7, 28, 2, 29, 7, 29, 2, 30, 7, 30, 2, 31, 7,
		31, 2, 32, 7, 32, 2, 33, 7, 33, 2, 34, 7, 34, 2, 35, 7, 35, 2, 36, 7, 36,
		1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		3, 1, 87, 8, 1, 1, 2, 1, 2, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 3,
		3, 98, 8, 3, 1, 3, 1, 3, 1, 4, 1, 4, 1, 4, 3, 4, 105, 8, 4, 1, 5, 1, 5,
		1, 5, 3, 5, 110, 8, 5, 1, 6, 1, 6, 1, 6, 1, 6, 1, 6, 5, 6, 117, 8, 6, 10,
		6, 12, 6, 120, 9, 6, 3, 6, 122, 8, 6, 1, 6, 1, 6, 1, 6, 1, 6, 1, 7, 1,
		7, 1, 7, 1, 7, 1, 8, 1, 8, 1, 8, 3, 8, 135, 8, 8, 1, 9, 1, 9, 1, 9, 5,
		9, 140, 8, 9, 10, 9, 12, 9, 143, 9, 9, 1, 10, 1, 10, 1, 11, 1, 11, 1, 11,
		1, 12, 1, 12, 1, 12, 1, 13, 1, 13, 1, 13, 5, 13, 156, 8, 13, 10, 13, 12,
		13, 159, 9, 13, 1, 13, 3, 13, 162, 8, 13, 3, 13, 164, 8, 13, 1, 13, 1,
		13, 1, 14, 3, 14, 169, 8, 14, 1, 14, 1, 14, 1, 14, 5, 14, 174, 8, 14, 10,
		14, 12, 14, 177, 9, 14, 1, 15, 1, 15, 1, 15, 1, 16, 1, 16, 3, 16, 184,
		8, 16, 1, 16, 3, 16, 187, 8, 16, 1, 17, 3, 17, 190, 8, 17, 1, 17, 1, 17,
		1, 17, 5, 17, 195, 8, 17, 10, 17, 12, 17, 198, 9, 17, 1, 18, 1, 18, 1,
		19, 1, 19, 1, 19, 1, 19, 5, 19, 206, 8, 19, 10, 19, 12, 19, 209, 9, 19,
		3, 19, 211, 8, 19, 1, 19, 1, 19, 1, 20, 1, 20, 1, 20, 1, 21, 1, 21, 1,
		21, 1, 21, 1, 21, 1, 21, 3, 21, 224, 8, 21, 1, 22, 1, 22, 1, 22, 5, 22,
		229, 8, 22, 10, 22, 12, 22, 232, 9, 22, 1, 23, 1, 23, 1, 23, 5, 23, 237,
		8, 23, 10, 23, 12, 23, 240, 9, 23, 1, 24, 1, 24, 1, 24, 1, 24, 1, 24, 1,
		24, 5, 24, 248, 8, 24, 10, 24, 12, 24, 251, 9, 24, 1, 25, 1, 25, 1, 25,
		1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 1, 25, 5, 25, 262, 8, 25, 10, 25, 12,
		25, 265, 9, 25, 1, 26, 1, 26, 4, 26, 269, 8, 26, 11, 26, 12, 26, 270, 1,
		26, 1, 26, 4, 26, 275, 8, 26, 11, 26, 12, 26, 276, 1, 26, 3, 26, 280, 8,
		26, 1, 27, 1, 27, 1, 27, 1, 27, 1, 27, 1, 27, 3, 27, 288, 8, 27, 1, 27,
		1, 27, 1, 27, 1, 27, 1, 27, 1, 27, 3, 27, 296, 8, 27, 1, 27, 1, 27, 1,
		27, 1, 27, 3, 27, 302, 8, 27, 1, 27, 1, 27, 1, 27, 5, 27, 307, 8, 27, 10,
		27, 12, 27, 310, 9, 27, 1, 28, 3, 28, 313, 8, 28, 1, 28, 1, 28, 3, 28,
		317, 8, 28, 1, 28, 1, 28, 1, 28, 3, 28, 322, 8, 28, 1, 28, 1, 28, 1, 28,
		1, 28, 1, 28, 1, 28, 1, 28, 3, 28, 331, 8, 28, 1, 28, 3, 28, 334, 8, 28,
		1, 28, 1, 28, 1, 28, 3, 28, 339, 8, 28, 1, 28, 3, 28, 342, 8, 28, 1, 28,
		1, 28, 3, 28, 346, 8, 28, 1, 28, 1, 28, 1, 28, 5, 28, 351, 8, 28, 10, 28,
		12, 28, 354, 9, 28, 1, 28, 1, 28, 3, 28, 358, 8, 28, 1, 28, 3, 28, 361,
		8, 28, 1, 28, 1, 28, 3, 28, 365, 8, 28, 1, 29, 1, 29, 1, 29, 5, 29, 370,
		8, 29, 10, 29, 12, 29, 373, 9, 29, 1, 30, 1, 30, 1, 30, 5, 30, 378, 8,
		30, 10, 30, 12, 30, 381, 9, 30, 1, 31, 1, 31, 1, 31, 1, 31, 1, 31, 1, 31,
		1, 31, 1, 31, 5, 31, 391, 8, 31, 10, 31, 12, 31, 394, 9, 31, 1, 32, 3,
		32, 397, 8, 32, 1, 32, 1, 32, 1, 33, 1, 33, 1, 33, 1, 33, 1, 33, 1, 33,
		1, 33, 1, 33, 5, 33, 409, 8, 33, 10, 33, 12, 33, 412, 9, 33, 1, 34, 1,
		34, 3, 34, 416, 8, 34, 1, 35, 3, 35, 419, 8, 35, 1, 35, 1, 35, 1, 36, 3,
		36, 424, 8, 36, 1, 36, 1, 36, 1, 36, 3, 36, 429, 8, 36, 1, 36, 1, 36, 1,
		36, 1, 36, 1, 36, 1, 36, 3, 36, 437, 8, 36, 1, 36, 0, 3, 48, 50, 54, 37,
		0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36,
		38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58, 60, 62, 64, 66, 68, 70, 72,
		0, 5, 1, 0, 1, 2, 2, 0, 42, 42, 50, 50, 1, 0, 15, 21, 1, 0, 37, 39, 2,
		0, 32, 32, 36, 36, 477, 0, 74, 1, 0, 0, 0, 2, 86, 1, 0, 0, 0, 4, 88, 1,
		0, 0, 0, 6, 90, 1, 0, 0, 0, 8, 101, 1, 0, 0, 0, 10, 106, 1, 0, 0, 0, 12,
		111, 1, 0, 0, 0, 14, 127, 1, 0, 0, 0, 16, 131, 1, 0, 0, 0, 18, 136, 1,
		0, 0, 0, 20, 144, 1, 0, 0, 0, 22, 146, 1, 0, 0, 0, 24, 149, 1, 0, 0, 0,
		26, 163, 1, 0, 0, 0, 28, 168, 1, 0, 0, 0, 30, 178, 1, 0, 0, 0, 32, 186,
		1, 0, 0, 0, 34, 189, 1, 0, 0, 0, 36, 199, 1, 0, 0, 0, 38, 201, 1, 0, 0,
		0, 40, 214, 1, 0, 0, 0, 42, 217, 1, 0, 0, 0, 44, 225, 1, 0, 0, 0, 46, 233,
		1, 0, 0, 0, 48, 241, 1, 0, 0, 0, 50, 252, 1, 0, 0, 0, 52, 279, 1, 0, 0,
		0, 54, 281, 1, 0, 0, 0, 56, 364, 1, 0, 0, 0, 58, 366, 1, 0, 0, 0, 60, 374,
		1, 0, 0, 0, 62, 382, 1, 0, 0, 0, 64, 396, 1, 0, 0, 0, 66, 400, 1, 0, 0,
		0, 68, 415, 1, 0, 0, 0, 70, 418, 1, 0, 0, 0, 72, 436, 1, 0, 0, 0, 74, 75,
		3, 2, 1, 0, 75, 76, 5, 0, 0, 1, 76, 1, 1, 0, 0, 0, 77, 87, 3, 4, 2, 0,
		78, 87, 3, 6, 3, 0, 79, 87, 3, 8, 4, 0, 80, 87, 3, 16, 8, 0, 81, 87, 3,
		18, 9, 0, 82, 87, 3, 22, 11, 0, 83, 87, 3, 24, 12, 0, 84, 87, 3, 26, 13,
		0, 85, 87, 3, 20, 10, 0, 86, 77, 1, 0, 0, 0, 86, 78, 1, 0, 0, 0, 86, 79,
		1, 0, 0, 0, 86, 80, 1, 0, 0, 0, 86, 81, 1, 0, 0, 0, 86, 82, 1, 0, 0, 0,
		86, 83, 1, 0, 0, 0, 86, 84, 1, 0, 0, 0, 86, 85, 1, 0, 0, 0, 87, 3, 1, 0,
		0, 0, 88, 89, 7, 0, 0, 0, 89, 5, 1, 0, 0, 0, 90, 97, 5, 3, 0, 0, 91, 92,
		3, 10, 5, 0, 92, 93, 5, 13, 0, 0, 93, 98, 1, 0, 0, 0, 94, 95, 3, 12, 6,
		0, 95, 96, 5, 12, 0, 0, 96, 98, 1, 0, 0, 0, 97, 91, 1, 0, 0, 0, 97, 94,
		1, 0, 0, 0, 98, 99, 1, 0, 0, 0, 99, 100, 3, 42, 21, 0, 100, 7, 1, 0, 0,
		0, 101, 104, 5, 4, 0, 0, 102, 105, 3, 10, 5, 0, 103, 105, 3, 12, 6, 0,
		104, 102, 1, 0, 0, 0, 104, 103, 1, 0, 0, 0, 105, 9, 1, 0, 0, 0, 106, 109,
		3, 28, 14, 0, 107, 108, 5, 35, 0, 0, 108, 110, 3, 32, 16, 0, 109, 107,
		1, 0, 0, 0, 109, 110, 1, 0, 0, 0, 110, 11, 1, 0, 0, 0, 111, 112, 3, 28,
		14, 0, 112, 121, 5, 28, 0, 0, 113, 118, 3, 14, 7, 0, 114, 115, 5, 31, 0,
		0, 115, 117, 3, 14, 7, 0, 116, 114, 1, 0, 0, 0, 117, 120, 1, 0, 0, 0, 118,
		116, 1, 0, 0, 0, 118, 119, 1, 0, 0, 0, 119, 122, 1, 0, 0, 0, 120, 118,
		1, 0, 0, 0, 121, 113, 1, 0, 0, 0, 121, 122, 1, 0, 0, 0, 122, 123, 1, 0,
		0, 0, 123, 124, 5, 29, 0, 0, 124, 125, 5, 35, 0, 0, 125, 126, 3, 32, 16,
		0, 126, 13, 1, 0, 0, 0, 127, 128, 5, 50, 0, 0, 128, 129, 5, 35, 0, 0, 129,
		130, 3, 32, 16, 0, 130, 15, 1, 0, 0, 0, 131, 134, 5, 5, 0, 0, 132, 135,
		3, 10, 5, 0, 133, 135, 3, 12, 6, 0, 134, 132, 1, 0, 0, 0, 134, 133, 1,
		0, 0, 0, 135, 17, 1, 0, 0, 0, 136, 141, 5, 10, 0, 0, 137, 140, 5, 11, 0,
		0, 138, 140, 5, 48, 0, 0, 139, 137, 1, 0, 0, 0, 139, 138, 1, 0, 0, 0, 140,
		143, 1, 0, 0, 0, 141, 139, 1, 0, 0, 0, 141, 142, 1, 0, 0, 0, 142, 19, 1,
		0, 0, 0, 143, 141, 1, 0, 0, 0, 144, 145, 1, 0, 0, 0, 145, 21, 1, 0, 0,
		0, 146, 147, 5, 6, 0, 0, 147, 148, 3, 42, 21, 0, 148, 23, 1, 0, 0, 0, 149,
		150, 5, 7, 0, 0, 150, 151, 3, 42, 21, 0, 151, 25, 1, 0, 0, 0, 152, 161,
		5, 8, 0, 0, 153, 157, 5, 11, 0, 0, 154, 156, 5, 11, 0, 0, 155, 154, 1,
		0, 0, 0, 156, 159, 1, 0, 0, 0, 157, 155, 1, 0, 0, 0, 157, 158, 1, 0, 0,
		0, 158, 160, 1, 0, 0, 0, 159, 157, 1, 0, 0, 0, 160, 162, 5, 9, 0, 0, 161,
		153, 1, 0, 0, 0, 161, 162, 1, 0, 0, 0, 162, 164, 1, 0, 0, 0, 163, 152,
		1, 0, 0, 0, 163, 164, 1, 0, 0, 0, 164, 165, 1, 0, 0, 0, 165, 166, 3, 42,
		21, 0, 166, 27, 1, 0, 0, 0, 167, 169, 5, 30, 0, 0, 168, 167, 1, 0, 0, 0,
		168, 169, 1, 0, 0, 0, 169, 170, 1, 0, 0, 0, 170, 175, 5, 50, 0, 0, 171,
		172, 5, 30, 0, 0, 172, 174, 5, 50, 0, 0, 173, 171, 1, 0, 0, 0, 174, 177,
		1, 0, 0, 0, 175, 173, 1, 0, 0, 0, 175, 176, 1, 0, 0, 0, 176, 29, 1, 0,
		0, 0, 177, 175, 1, 0, 0, 0, 178, 179, 3, 32, 16, 0, 179, 180, 5, 0, 0,
		1, 180, 31, 1, 0, 0, 0, 181, 183, 3, 34, 17, 0, 182, 184, 3, 38, 19, 0,
		183, 182, 1, 0, 0, 0, 183, 184, 1, 0, 0, 0, 184, 187, 1, 0, 0, 0, 185,
		187, 3, 36, 18, 0, 186, 181, 1, 0, 0, 0, 186, 185, 1, 0, 0, 0, 187, 33,
		1, 0, 0, 0, 188, 190, 5, 30, 0, 0, 189, 188, 1, 0, 0, 0, 189, 190, 1, 0,
		0, 0, 190, 191, 1, 0, 0, 0, 191, 196, 7, 1, 0, 0, 192, 193, 5, 30, 0, 0,
		193, 195, 5, 50, 0, 0, 194, 192, 1, 0, 0, 0, 195, 198, 1, 0, 0, 0, 196,
		194, 1, 0, 0, 0, 196, 197, 1, 0, 0, 0, 197, 35, 1, 0, 0, 0, 198, 196, 1,
		0, 0, 0, 199, 200, 5, 14, 0, 0, 200, 37, 1, 0, 0, 0, 201, 210, 5, 18, 0,
		0, 202, 207, 3, 32, 16, 0, 203, 204, 5, 31, 0, 0, 204, 206, 3, 32, 16,
		0, 205, 203, 1, 0, 0, 0, 206, 209, 1, 0, 0, 0, 207, 205, 1, 0, 0, 0, 207,
		208, 1, 0, 0, 0, 208, 211, 1, 0, 0, 0, 209, 207, 1, 0, 0, 0, 210, 202,
		1, 0, 0, 0, 210, 211, 1, 0, 0, 0, 211, 212, 1, 0, 0, 0, 212, 213, 5, 21,
		0, 0, 213, 39, 1, 0, 0, 0, 214, 215, 3, 42, 21, 0, 215, 216, 5, 0, 0, 1,
		216, 41, 1, 0, 0, 0, 217, 223, 3, 44, 22, 0, 218, 219, 5, 34, 0, 0, 219,
		220, 3, 44, 22, 0, 220, 221, 5, 35, 0, 0, 221, 222, 3, 42, 21, 0, 222,
		224, 1, 0, 0, 0, 223, 218, 1, 0, 0, 0, 223, 224, 1, 0, 0, 0, 224, 43, 1,
		0, 0, 0, 225, 230, 3, 46, 23, 0, 226, 227, 5, 23, 0, 0, 227, 229, 3, 46,
		23, 0, 228, 226, 1, 0, 0, 0, 229, 232, 1, 0, 0, 0, 230, 228, 1, 0, 0, 0,
		230, 231, 1, 0, 0, 0, 231, 45, 1, 0, 0, 0, 232, 230, 1, 0, 0, 0, 233, 238,
		3, 48, 24, 0, 234, 235, 5, 22, 0, 0, 235, 237, 3, 48, 24, 0, 236, 234,
		1, 0, 0, 0, 237, 240, 1, 0, 0, 0, 238, 236, 1, 0, 0, 0, 238, 239, 1, 0,
		0, 0, 239, 47, 1, 0, 0, 0, 240, 238, 1, 0, 0, 0, 241, 242, 6, 24, -1, 0,
		242, 243, 3, 50, 25, 0, 243, 249, 1, 0, 0, 0, 244, 245, 10, 1, 0, 0, 245,
		246, 7, 2, 0, 0, 246, 248, 3, 48, 24, 2, 247, 244, 1, 0, 0, 0, 248, 251,
		1, 0, 0, 0, 249, 247, 1, 0, 0, 0, 249, 250, 1, 0, 0, 0, 250, 49, 1, 0,
		0, 0, 251, 249, 1, 0, 0, 0, 252, 253, 6, 25, -1, 0, 253, 254, 3, 52, 26,
		0, 254, 263, 1, 0, 0, 0, 255, 256, 10, 2, 0, 0, 256, 257, 7, 3, 0, 0, 257,
		262, 3, 50, 25, 3, 258, 259, 10, 1, 0, 0, 259, 260, 7, 4, 0, 0, 260, 262,
		3, 50, 25, 2, 261, 255, 1, 0, 0, 0, 261, 258, 1, 0, 0, 0, 262, 265, 1,
		0, 0, 0, 263, 261, 1, 0, 0, 0, 263, 264, 1, 0, 0, 0, 264, 51, 1, 0, 0,
		0, 265, 263, 1, 0, 0, 0, 266, 280, 3, 54, 27, 0, 267, 269, 5, 33, 0, 0,
		268, 267, 1, 0, 0, 0, 269, 270, 1, 0, 0, 0, 270, 268, 1, 0, 0, 0, 270,
		271, 1, 0, 0, 0, 271, 272, 1, 0, 0, 0, 272, 280, 3, 54, 27, 0, 273, 275,
		5, 32, 0, 0, 274, 273, 1, 0, 0, 0, 275, 276, 1, 0, 0, 0, 276, 274, 1, 0,
		0, 0, 276, 277, 1, 0, 0, 0, 277, 278, 1, 0, 0, 0, 278, 280, 3, 54, 27,
		0, 279, 266, 1, 0, 0, 0, 279, 268, 1, 0, 0, 0, 279, 274, 1, 0, 0, 0, 280,
		53, 1, 0, 0, 0, 281, 282, 6, 27, -1, 0, 282, 283, 3, 56, 28, 0, 283, 308,
		1, 0, 0, 0, 284, 285, 10, 3, 0, 0, 285, 287, 5, 30, 0, 0, 286, 288, 5,
		34, 0, 0, 287, 286, 1, 0, 0, 0, 287, 288, 1, 0, 0, 0, 288, 289, 1, 0, 0,
		0, 289, 307, 3, 68, 34, 0, 290, 291, 10, 2, 0, 0, 291, 292, 5, 30, 0, 0,
		292, 293, 5, 50, 0, 0, 293, 295, 5, 28, 0, 0, 294, 296, 3, 58, 29, 0, 295,
		294, 1, 0, 0, 0, 295, 296, 1, 0, 0, 0, 296, 297, 1, 0, 0, 0, 297, 307,
		5, 29, 0, 0, 298, 299, 10, 1, 0, 0, 299, 301, 5, 24, 0, 0, 300, 302, 5,
		34, 0, 0, 301, 300, 1, 0, 0, 0, 301, 302, 1, 0, 0, 0, 302, 303, 1, 0, 0,
		0, 303, 304, 3, 42, 21, 0, 304, 305, 5, 25, 0, 0, 305, 307, 1, 0, 0, 0,
		306, 284, 1, 0, 0, 0, 306, 290, 1, 0, 0, 0, 306, 298, 1, 0, 0, 0, 307,
		310, 1, 0, 0, 0, 308, 306, 1, 0, 0, 0, 308, 309, 1, 0, 0, 0, 309, 55, 1,
		0, 0, 0, 310, 308, 1, 0, 0, 0, 311, 313, 5, 30, 0, 0, 312, 311, 1, 0, 0,
		0, 312, 313, 1, 0, 0, 0, 313, 314, 1, 0, 0, 0, 314, 365, 5, 50, 0, 0, 315,
		317, 5, 30, 0, 0, 316, 315, 1, 0, 0, 0, 316, 317, 1, 0, 0, 0, 317, 318,
		1, 0, 0, 0, 318, 319, 5, 50, 0, 0, 319, 321, 5, 28, 0, 0, 320, 322, 3,
		58, 29, 0, 321, 320, 1, 0, 0, 0, 321, 322, 1, 0, 0, 0, 322, 323, 1, 0,
		0, 0, 323, 365, 5, 29, 0, 0, 324, 325, 5, 28, 0, 0, 325, 326, 3, 42, 21,
		0, 326, 327, 5, 29, 0, 0, 327, 365, 1, 0, 0, 0, 328, 330, 5, 24, 0, 0,
		329, 331, 3, 60, 30, 0, 330, 329, 1, 0, 0, 0, 330, 331, 1, 0, 0, 0, 331,
		333, 1, 0, 0, 0, 332, 334, 5, 31, 0, 0, 333, 332, 1, 0, 0, 0, 333, 334,
		1, 0, 0, 0, 334, 335, 1, 0, 0, 0, 335, 365, 5, 25, 0, 0, 336, 338, 5, 26,
		0, 0, 337, 339, 3, 66, 33, 0, 338, 337, 1, 0, 0, 0, 338, 339, 1, 0, 0,
		0, 339, 341, 1, 0, 0, 0, 340, 342, 5, 31, 0, 0, 341, 340, 1, 0, 0, 0, 341,
		342, 1, 0, 0, 0, 342, 343, 1, 0, 0, 0, 343, 365, 5, 27, 0, 0, 344, 346,
		5, 30, 0, 0, 345, 344, 1, 0, 0, 0, 345, 346, 1, 0, 0, 0, 346, 347, 1, 0,
		0, 0, 347, 352, 5, 50, 0, 0, 348, 349, 5, 30, 0, 0, 349, 351, 5, 50, 0,
		0, 350, 348, 1, 0, 0, 0, 351, 354, 1, 0, 0, 0, 352, 350, 1, 0, 0, 0, 352,
		353, 1, 0, 0, 0, 353, 355, 1, 0, 0, 0, 354, 352, 1, 0, 0, 0, 355, 357,
		5, 26, 0, 0, 356, 358, 3, 62, 31, 0, 357, 356, 1, 0, 0, 0, 357, 358, 1,
		0, 0, 0, 358, 360, 1, 0, 0, 0, 359, 361, 5, 31, 0, 0, 360, 359, 1, 0, 0,
		0, 360, 361, 1, 0, 0, 0, 361, 362, 1, 0, 0, 0, 362, 365, 5, 27, 0, 0, 363,
		365, 3, 72, 36, 0, 364, 312, 1, 0, 0, 0, 364, 316, 1, 0, 0, 0, 364, 324,
		1, 0, 0, 0, 364, 328, 1, 0, 0, 0, 364, 336, 1, 0, 0, 0, 364, 345, 1, 0,
		0, 0, 364, 363, 1, 0, 0, 0, 365, 57, 1, 0, 0, 0, 366, 371, 3, 42, 21, 0,
		367, 368, 5, 31, 0, 0, 368, 370, 3, 42, 21, 0, 369, 367, 1, 0, 0, 0, 370,
		373, 1, 0, 0, 0, 371, 369, 1, 0, 0, 0, 371, 372, 1, 0, 0, 0, 372, 59, 1,
		0, 0, 0, 373, 371, 1, 0, 0, 0, 374, 379, 3, 70, 35, 0, 375, 376, 5, 31,
		0, 0, 376, 378, 3, 70, 35, 0, 377, 375, 1, 0, 0, 0, 378, 381, 1, 0, 0,
		0, 379, 377, 1, 0, 0, 0, 379, 380, 1, 0, 0, 0, 380, 61, 1, 0, 0, 0, 381,
		379, 1, 0, 0, 0, 382, 383, 3, 64, 32, 0, 383, 384, 5, 35, 0, 0, 384, 392,
		3, 42, 21, 0, 385, 386, 5, 31, 0, 0, 386, 387, 3, 64, 32, 0, 387, 388,
		5, 35, 0, 0, 388, 389, 3, 42, 21, 0, 389, 391, 1, 0, 0, 0, 390, 385, 1,
		0, 0, 0, 391, 394, 1, 0, 0, 0, 392, 390, 1, 0, 0, 0, 392, 393, 1, 0, 0,
		0, 393, 63, 1, 0, 0, 0, 394, 392, 1, 0, 0, 0, 395, 397, 5, 34, 0, 0, 396,
		395, 1, 0, 0, 0, 396, 397, 1, 0, 0, 0, 397, 398, 1, 0, 0, 0, 398, 399,
		3, 68, 34, 0, 399, 65, 1, 0, 0, 0, 400, 401, 3, 70, 35, 0, 401, 402, 5,
		35, 0, 0, 402, 410, 3, 42, 21, 0, 403, 404, 5, 31, 0, 0, 404, 405, 3, 70,
		35, 0, 405, 406, 5, 35, 0, 0, 406, 407, 3, 42, 21, 0, 407, 409, 1, 0, 0,
		0, 408, 403, 1, 0, 0, 0, 409, 412, 1, 0, 0, 0, 410, 408, 1, 0, 0, 0, 410,
		411, 1, 0, 0, 0, 411, 67, 1, 0, 0, 0, 412, 410, 1, 0, 0, 0, 413, 416, 5,
		50, 0, 0, 414, 416, 5, 51, 0, 0, 415, 413, 1, 0, 0, 0, 415, 414, 1, 0,
		0, 0, 416, 69, 1, 0, 0, 0, 417, 419, 5, 34, 0, 0, 418, 417, 1, 0, 0, 0,
		418, 419, 1, 0, 0, 0, 419, 420, 1, 0, 0, 0, 420, 421, 3, 42, 21, 0, 421,
		71, 1, 0, 0, 0, 422, 424, 5, 32, 0, 0, 423, 422, 1, 0, 0, 0, 423, 424,
		1, 0, 0, 0, 424, 425, 1, 0, 0, 0, 425, 437, 5, 46, 0, 0, 426, 437, 5, 47,
		0, 0, 427, 429, 5, 32, 0, 0, 428, 427, 1, 0, 0, 0, 428, 429, 1, 0, 0, 0,
		429, 430, 1, 0, 0, 0, 430, 437, 5, 45, 0, 0, 431, 437, 5, 48, 0, 0, 432,
		437, 5, 49, 0, 0, 433, 437, 5, 40, 0, 0, 434, 437, 5, 41, 0, 0, 435, 437,
		5, 42, 0, 0, 436, 423, 1, 0, 0, 0, 436, 426, 1, 0, 0, 0, 436, 428, 1, 0,
		0, 0, 436, 431, 1, 0, 0, 0, 436, 432, 1, 0, 0, 0, 436, 433, 1, 0, 0, 0,
		436, 434, 1, 0, 0, 0, 436, 435, 1, 0, 0, 0, 437, 73, 1, 0, 0, 0, 56, 86,
		97, 104, 109, 118, 121, 134, 139, 141, 157, 161, 163, 168, 175, 183, 186,
		189, 196, 207, 210, 223, 230, 238, 249, 261, 263, 270, 276, 279, 287, 295,
		301, 306, 308, 312, 316, 321, 330, 333, 338, 341, 345, 352, 357, 360, 364,
		371, 379, 392, 396, 410, 415, 418, 423, 428, 436,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// CommandsParserInit initializes any static state used to implement CommandsParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewCommandsParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func CommandsParserInit() {
	staticData := &CommandsParserStaticData
	staticData.once.Do(commandsParserInit)
}

// NewCommandsParser produces a new parser instance for the optional input antlr.TokenStream.
func NewCommandsParser(input antlr.TokenStream) *CommandsParser {
	CommandsParserInit()
	this := new(CommandsParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &CommandsParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
	this.GrammarFileName = "Commands.g4"

	return this
}

// CommandsParser tokens.
const (
	CommandsParserEOF             = antlr.TokenEOF
	CommandsParserT__0            = 1
	CommandsParserT__1            = 2
	CommandsParserT__2            = 3
	CommandsParserT__3            = 4
	CommandsParserT__4            = 5
	CommandsParserT__5            = 6
	CommandsParserT__6            = 7
	CommandsParserT__7            = 8
	CommandsParserT__8            = 9
	CommandsParserCOMMAND         = 10
	CommandsParserFLAG            = 11
	CommandsParserARROW           = 12
	CommandsParserEQUAL_ASSIGN    = 13
	CommandsParserPARAM_SPECIFIER = 14
	CommandsParserEQUALS          = 15
	CommandsParserNOT_EQUALS      = 16
	CommandsParserIN              = 17
	CommandsParserLESS            = 18
	CommandsParserLESS_EQUALS     = 19
	CommandsParserGREATER_EQUALS  = 20
	CommandsParserGREATER         = 21
	CommandsParserLOGICAL_AND     = 22
	CommandsParserLOGICAL_OR      = 23
	CommandsParserLBRACKET        = 24
	CommandsParserRPRACKET        = 25
	CommandsParserLBRACE          = 26
	CommandsParserRBRACE          = 27
	CommandsParserLPAREN          = 28
	CommandsParserRPAREN          = 29
	CommandsParserDOT             = 30
	CommandsParserCOMMA           = 31
	CommandsParserMINUS           = 32
	CommandsParserEXCLAM          = 33
	CommandsParserQUESTIONMARK    = 34
	CommandsParserCOLON           = 35
	CommandsParserPLUS            = 36
	CommandsParserSTAR            = 37
	CommandsParserSLASH           = 38
	CommandsParserPERCENT         = 39
	CommandsParserCEL_TRUE        = 40
	CommandsParserCEL_FALSE       = 41
	CommandsParserNUL             = 42
	CommandsParserWHITESPACE      = 43
	CommandsParserCOMMENT         = 44
	CommandsParserNUM_FLOAT       = 45
	CommandsParserNUM_INT         = 46
	CommandsParserNUM_UINT        = 47
	CommandsParserSTRING          = 48
	CommandsParserBYTES           = 49
	CommandsParserIDENTIFIER      = 50
	CommandsParserESC_IDENTIFIER  = 51
)

// CommandsParser rules.
const (
	CommandsParserRULE_startCommand         = 0
	CommandsParserRULE_command              = 1
	CommandsParserRULE_help                 = 2
	CommandsParserRULE_let                  = 3
	CommandsParserRULE_declare              = 4
	CommandsParserRULE_varDecl              = 5
	CommandsParserRULE_fnDecl               = 6
	CommandsParserRULE_param                = 7
	CommandsParserRULE_delete               = 8
	CommandsParserRULE_simple               = 9
	CommandsParserRULE_empty                = 10
	CommandsParserRULE_compile              = 11
	CommandsParserRULE_parse                = 12
	CommandsParserRULE_exprCmd              = 13
	CommandsParserRULE_qualId               = 14
	CommandsParserRULE_startType            = 15
	CommandsParserRULE_type                 = 16
	CommandsParserRULE_typeId               = 17
	CommandsParserRULE_paramId              = 18
	CommandsParserRULE_typeParamList        = 19
	CommandsParserRULE_start                = 20
	CommandsParserRULE_expr                 = 21
	CommandsParserRULE_conditionalOr        = 22
	CommandsParserRULE_conditionalAnd       = 23
	CommandsParserRULE_relation             = 24
	CommandsParserRULE_calc                 = 25
	CommandsParserRULE_unary                = 26
	CommandsParserRULE_member               = 27
	CommandsParserRULE_primary              = 28
	CommandsParserRULE_exprList             = 29
	CommandsParserRULE_listInit             = 30
	CommandsParserRULE_fieldInitializerList = 31
	CommandsParserRULE_optField             = 32
	CommandsParserRULE_mapInitializerList   = 33
	CommandsParserRULE_escapeIdent          = 34
	CommandsParserRULE_optExpr              = 35
	CommandsParserRULE_literal              = 36
)

// IStartCommandContext is an interface to support dynamic dispatch.
type IStartCommandContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Command() ICommandContext
	EOF() antlr.TerminalNode

	// IsStartCommandContext differentiates from other interfaces.
	IsStartCommandContext()
}

type StartCommandContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStartCommandContext() *StartCommandContext {
	var p = new(StartCommandContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_startCommand
	return p
}

func InitEmptyStartCommandContext(p *StartCommandContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_startCommand
}

func (*StartCommandContext) IsStartCommandContext() {}

func NewStartCommandContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StartCommandContext {
	var p = new(StartCommandContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_startCommand

	return p
}

func (s *StartCommandContext) GetParser() antlr.Parser { return s.parser }

func (s *StartCommandContext) Command() ICommandContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICommandContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ICommandContext)
}

func (s *StartCommandContext) EOF() antlr.TerminalNode {
	return s.GetToken(CommandsParserEOF, 0)
}

func (s *StartCommandContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StartCommandContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StartCommandContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterStartCommand(s)
	}
}

func (s *StartCommandContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitStartCommand(s)
	}
}

func (s *StartCommandContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitStartCommand(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) StartCommand() (localctx IStartCommandContext) {
	localctx = NewStartCommandContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, CommandsParserRULE_startCommand)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(74)
		p.Command()
	}
	{
		p.SetState(75)
		p.Match(CommandsParserEOF)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ICommandContext is an interface to support dynamic dispatch.
type ICommandContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Help() IHelpContext
	Let() ILetContext
	Declare() IDeclareContext
	Delete_() IDeleteContext
	Simple() ISimpleContext
	Compile() ICompileContext
	Parse() IParseContext
	ExprCmd() IExprCmdContext
	Empty() IEmptyContext

	// IsCommandContext differentiates from other interfaces.
	IsCommandContext()
}

type CommandContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyCommandContext() *CommandContext {
	var p = new(CommandContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_command
	return p
}

func InitEmptyCommandContext(p *CommandContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_command
}

func (*CommandContext) IsCommandContext() {}

func NewCommandContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CommandContext {
	var p = new(CommandContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_command

	return p
}

func (s *CommandContext) GetParser() antlr.Parser { return s.parser }

func (s *CommandContext) Help() IHelpContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IHelpContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IHelpContext)
}

func (s *CommandContext) Let() ILetContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ILetContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ILetContext)
}

func (s *CommandContext) Declare() IDeclareContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IDeclareContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IDeclareContext)
}

func (s *CommandContext) Delete_() IDeleteContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IDeleteContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IDeleteContext)
}

func (s *CommandContext) Simple() ISimpleContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ISimpleContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ISimpleContext)
}

func (s *CommandContext) Compile() ICompileContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICompileContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ICompileContext)
}

func (s *CommandContext) Parse() IParseContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IParseContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IParseContext)
}

func (s *CommandContext) ExprCmd() IExprCmdContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprCmdContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprCmdContext)
}

func (s *CommandContext) Empty() IEmptyContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEmptyContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEmptyContext)
}

func (s *CommandContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CommandContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *CommandContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterCommand(s)
	}
}

func (s *CommandContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitCommand(s)
	}
}

func (s *CommandContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitCommand(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Command() (localctx ICommandContext) {
	localctx = NewCommandContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, CommandsParserRULE_command)
	p.SetState(86)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case CommandsParserT__0, CommandsParserT__1:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(77)
			p.Help()
		}

	case CommandsParserT__2:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(78)
			p.Let()
		}

	case CommandsParserT__3:
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(79)
			p.Declare()
		}

	case CommandsParserT__4:
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(80)
			p.Delete_()
		}

	case CommandsParserCOMMAND:
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(81)
			p.Simple()
		}

	case CommandsParserT__5:
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(82)
			p.Compile()
		}

	case CommandsParserT__6:
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(83)
			p.Parse()
		}

	case CommandsParserT__7, CommandsParserLBRACKET, CommandsParserLBRACE, CommandsParserLPAREN, CommandsParserDOT, CommandsParserMINUS, CommandsParserEXCLAM, CommandsParserCEL_TRUE, CommandsParserCEL_FALSE, CommandsParserNUL, CommandsParserNUM_FLOAT, CommandsParserNUM_INT, CommandsParserNUM_UINT, CommandsParserSTRING, CommandsParserBYTES, CommandsParserIDENTIFIER:
		p.EnterOuterAlt(localctx, 8)
		{
			p.SetState(84)
			p.ExprCmd()
		}

	case CommandsParserEOF:
		p.EnterOuterAlt(localctx, 9)
		{
			p.SetState(85)
			p.Empty()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IHelpContext is an interface to support dynamic dispatch.
type IHelpContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsHelpContext differentiates from other interfaces.
	IsHelpContext()
}

type HelpContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyHelpContext() *HelpContext {
	var p = new(HelpContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_help
	return p
}

func InitEmptyHelpContext(p *HelpContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_help
}

func (*HelpContext) IsHelpContext() {}

func NewHelpContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *HelpContext {
	var p = new(HelpContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_help

	return p
}

func (s *HelpContext) GetParser() antlr.Parser { return s.parser }
func (s *HelpContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *HelpContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *HelpContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterHelp(s)
	}
}

func (s *HelpContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitHelp(s)
	}
}

func (s *HelpContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitHelp(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Help() (localctx IHelpContext) {
	localctx = NewHelpContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, CommandsParserRULE_help)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(88)
		_la = p.GetTokenStream().LA(1)

		if !(_la == CommandsParserT__0 || _la == CommandsParserT__1) {
			p.GetErrorHandler().RecoverInline(p)
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ILetContext is an interface to support dynamic dispatch.
type ILetContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetVar_ returns the var_ rule contexts.
	GetVar_() IVarDeclContext

	// GetFn returns the fn rule contexts.
	GetFn() IFnDeclContext

	// GetE returns the e rule contexts.
	GetE() IExprContext

	// SetVar_ sets the var_ rule contexts.
	SetVar_(IVarDeclContext)

	// SetFn sets the fn rule contexts.
	SetFn(IFnDeclContext)

	// SetE sets the e rule contexts.
	SetE(IExprContext)

	// Getter signatures
	Expr() IExprContext
	EQUAL_ASSIGN() antlr.TerminalNode
	ARROW() antlr.TerminalNode
	VarDecl() IVarDeclContext
	FnDecl() IFnDeclContext

	// IsLetContext differentiates from other interfaces.
	IsLetContext()
}

type LetContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	var_   IVarDeclContext
	fn     IFnDeclContext
	e      IExprContext
}

func NewEmptyLetContext() *LetContext {
	var p = new(LetContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_let
	return p
}

func InitEmptyLetContext(p *LetContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_let
}

func (*LetContext) IsLetContext() {}

func NewLetContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *LetContext {
	var p = new(LetContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_let

	return p
}

func (s *LetContext) GetParser() antlr.Parser { return s.parser }

func (s *LetContext) GetVar_() IVarDeclContext { return s.var_ }

func (s *LetContext) GetFn() IFnDeclContext { return s.fn }

func (s *LetContext) GetE() IExprContext { return s.e }

func (s *LetContext) SetVar_(v IVarDeclContext) { s.var_ = v }

func (s *LetContext) SetFn(v IFnDeclContext) { s.fn = v }

func (s *LetContext) SetE(v IExprContext) { s.e = v }

func (s *LetContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *LetContext) EQUAL_ASSIGN() antlr.TerminalNode {
	return s.GetToken(CommandsParserEQUAL_ASSIGN, 0)
}

func (s *LetContext) ARROW() antlr.TerminalNode {
	return s.GetToken(CommandsParserARROW, 0)
}

func (s *LetContext) VarDecl() IVarDeclContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IVarDeclContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IVarDeclContext)
}

func (s *LetContext) FnDecl() IFnDeclContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnDeclContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnDeclContext)
}

func (s *LetContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LetContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *LetContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterLet(s)
	}
}

func (s *LetContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitLet(s)
	}
}

func (s *LetContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitLet(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Let() (localctx ILetContext) {
	localctx = NewLetContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, CommandsParserRULE_let)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(90)
		p.Match(CommandsParserT__2)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(97)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 1, p.GetParserRuleContext()) {
	case 1:
		{
			p.SetState(91)

			var _x = p.VarDecl()

			localctx.(*LetContext).var_ = _x
		}
		{
			p.SetState(92)
			p.Match(CommandsParserEQUAL_ASSIGN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 2:
		{
			p.SetState(94)

			var _x = p.FnDecl()

			localctx.(*LetContext).fn = _x
		}
		{
			p.SetState(95)
			p.Match(CommandsParserARROW)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}
	{
		p.SetState(99)

		var _x = p.Expr()

		localctx.(*LetContext).e = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IDeclareContext is an interface to support dynamic dispatch.
type IDeclareContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetVar_ returns the var_ rule contexts.
	GetVar_() IVarDeclContext

	// GetFn returns the fn rule contexts.
	GetFn() IFnDeclContext

	// SetVar_ sets the var_ rule contexts.
	SetVar_(IVarDeclContext)

	// SetFn sets the fn rule contexts.
	SetFn(IFnDeclContext)

	// Getter signatures
	VarDecl() IVarDeclContext
	FnDecl() IFnDeclContext

	// IsDeclareContext differentiates from other interfaces.
	IsDeclareContext()
}

type DeclareContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	var_   IVarDeclContext
	fn     IFnDeclContext
}

func NewEmptyDeclareContext() *DeclareContext {
	var p = new(DeclareContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_declare
	return p
}

func InitEmptyDeclareContext(p *DeclareContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_declare
}

func (*DeclareContext) IsDeclareContext() {}

func NewDeclareContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DeclareContext {
	var p = new(DeclareContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_declare

	return p
}

func (s *DeclareContext) GetParser() antlr.Parser { return s.parser }

func (s *DeclareContext) GetVar_() IVarDeclContext { return s.var_ }

func (s *DeclareContext) GetFn() IFnDeclContext { return s.fn }

func (s *DeclareContext) SetVar_(v IVarDeclContext) { s.var_ = v }

func (s *DeclareContext) SetFn(v IFnDeclContext) { s.fn = v }

func (s *DeclareContext) VarDecl() IVarDeclContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IVarDeclContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IVarDeclContext)
}

func (s *DeclareContext) FnDecl() IFnDeclContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnDeclContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnDeclContext)
}

func (s *DeclareContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DeclareContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DeclareContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterDeclare(s)
	}
}

func (s *DeclareContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitDeclare(s)
	}
}

func (s *DeclareContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitDeclare(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Declare() (localctx IDeclareContext) {
	localctx = NewDeclareContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, CommandsParserRULE_declare)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(101)
		p.Match(CommandsParserT__3)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(104)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 2, p.GetParserRuleContext()) {
	case 1:
		{
			p.SetState(102)

			var _x = p.VarDecl()

			localctx.(*DeclareContext).var_ = _x
		}

	case 2:
		{
			p.SetState(103)

			var _x = p.FnDecl()

			localctx.(*DeclareContext).fn = _x
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IVarDeclContext is an interface to support dynamic dispatch.
type IVarDeclContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetId returns the id rule contexts.
	GetId() IQualIdContext

	// GetT returns the t rule contexts.
	GetT() ITypeContext

	// SetId sets the id rule contexts.
	SetId(IQualIdContext)

	// SetT sets the t rule contexts.
	SetT(ITypeContext)

	// Getter signatures
	QualId() IQualIdContext
	COLON() antlr.TerminalNode
	Type_() ITypeContext

	// IsVarDeclContext differentiates from other interfaces.
	IsVarDeclContext()
}

type VarDeclContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	id     IQualIdContext
	t      ITypeContext
}

func NewEmptyVarDeclContext() *VarDeclContext {
	var p = new(VarDeclContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_varDecl
	return p
}

func InitEmptyVarDeclContext(p *VarDeclContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_varDecl
}

func (*VarDeclContext) IsVarDeclContext() {}

func NewVarDeclContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *VarDeclContext {
	var p = new(VarDeclContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_varDecl

	return p
}

func (s *VarDeclContext) GetParser() antlr.Parser { return s.parser }

func (s *VarDeclContext) GetId() IQualIdContext { return s.id }

func (s *VarDeclContext) GetT() ITypeContext { return s.t }

func (s *VarDeclContext) SetId(v IQualIdContext) { s.id = v }

func (s *VarDeclContext) SetT(v ITypeContext) { s.t = v }

func (s *VarDeclContext) QualId() IQualIdContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IQualIdContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IQualIdContext)
}

func (s *VarDeclContext) COLON() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOLON, 0)
}

func (s *VarDeclContext) Type_() ITypeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeContext)
}

func (s *VarDeclContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *VarDeclContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *VarDeclContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterVarDecl(s)
	}
}

func (s *VarDeclContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitVarDecl(s)
	}
}

func (s *VarDeclContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitVarDecl(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) VarDecl() (localctx IVarDeclContext) {
	localctx = NewVarDeclContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, CommandsParserRULE_varDecl)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(106)

		var _x = p.QualId()

		localctx.(*VarDeclContext).id = _x
	}
	p.SetState(109)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserCOLON {
		{
			p.SetState(107)
			p.Match(CommandsParserCOLON)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(108)

			var _x = p.Type_()

			localctx.(*VarDeclContext).t = _x
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IFnDeclContext is an interface to support dynamic dispatch.
type IFnDeclContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetId returns the id rule contexts.
	GetId() IQualIdContext

	// Get_param returns the _param rule contexts.
	Get_param() IParamContext

	// GetRType returns the rType rule contexts.
	GetRType() ITypeContext

	// SetId sets the id rule contexts.
	SetId(IQualIdContext)

	// Set_param sets the _param rule contexts.
	Set_param(IParamContext)

	// SetRType sets the rType rule contexts.
	SetRType(ITypeContext)

	// GetParams returns the params rule context list.
	GetParams() []IParamContext

	// SetParams sets the params rule context list.
	SetParams([]IParamContext)

	// Getter signatures
	LPAREN() antlr.TerminalNode
	RPAREN() antlr.TerminalNode
	COLON() antlr.TerminalNode
	QualId() IQualIdContext
	Type_() ITypeContext
	AllParam() []IParamContext
	Param(i int) IParamContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsFnDeclContext differentiates from other interfaces.
	IsFnDeclContext()
}

type FnDeclContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	id     IQualIdContext
	_param IParamContext
	params []IParamContext
	rType  ITypeContext
}

func NewEmptyFnDeclContext() *FnDeclContext {
	var p = new(FnDeclContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_fnDecl
	return p
}

func InitEmptyFnDeclContext(p *FnDeclContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_fnDecl
}

func (*FnDeclContext) IsFnDeclContext() {}

func NewFnDeclContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FnDeclContext {
	var p = new(FnDeclContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_fnDecl

	return p
}

func (s *FnDeclContext) GetParser() antlr.Parser { return s.parser }

func (s *FnDeclContext) GetId() IQualIdContext { return s.id }

func (s *FnDeclContext) Get_param() IParamContext { return s._param }

func (s *FnDeclContext) GetRType() ITypeContext { return s.rType }

func (s *FnDeclContext) SetId(v IQualIdContext) { s.id = v }

func (s *FnDeclContext) Set_param(v IParamContext) { s._param = v }

func (s *FnDeclContext) SetRType(v ITypeContext) { s.rType = v }

func (s *FnDeclContext) GetParams() []IParamContext { return s.params }

func (s *FnDeclContext) SetParams(v []IParamContext) { s.params = v }

func (s *FnDeclContext) LPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserLPAREN, 0)
}

func (s *FnDeclContext) RPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserRPAREN, 0)
}

func (s *FnDeclContext) COLON() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOLON, 0)
}

func (s *FnDeclContext) QualId() IQualIdContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IQualIdContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IQualIdContext)
}

func (s *FnDeclContext) Type_() ITypeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeContext)
}

func (s *FnDeclContext) AllParam() []IParamContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IParamContext); ok {
			len++
		}
	}

	tst := make([]IParamContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IParamContext); ok {
			tst[i] = t.(IParamContext)
			i++
		}
	}

	return tst
}

func (s *FnDeclContext) Param(i int) IParamContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IParamContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IParamContext)
}

func (s *FnDeclContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOMMA)
}

func (s *FnDeclContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, i)
}

func (s *FnDeclContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FnDeclContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FnDeclContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterFnDecl(s)
	}
}

func (s *FnDeclContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitFnDecl(s)
	}
}

func (s *FnDeclContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitFnDecl(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) FnDecl() (localctx IFnDeclContext) {
	localctx = NewFnDeclContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, CommandsParserRULE_fnDecl)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(111)

		var _x = p.QualId()

		localctx.(*FnDeclContext).id = _x
	}
	{
		p.SetState(112)
		p.Match(CommandsParserLPAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(121)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserIDENTIFIER {
		{
			p.SetState(113)

			var _x = p.Param()

			localctx.(*FnDeclContext)._param = _x
		}
		localctx.(*FnDeclContext).params = append(localctx.(*FnDeclContext).params, localctx.(*FnDeclContext)._param)
		p.SetState(118)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == CommandsParserCOMMA {
			{
				p.SetState(114)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(115)

				var _x = p.Param()

				localctx.(*FnDeclContext)._param = _x
			}
			localctx.(*FnDeclContext).params = append(localctx.(*FnDeclContext).params, localctx.(*FnDeclContext)._param)

			p.SetState(120)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	}
	{
		p.SetState(123)
		p.Match(CommandsParserRPAREN)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(124)
		p.Match(CommandsParserCOLON)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(125)

		var _x = p.Type_()

		localctx.(*FnDeclContext).rType = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IParamContext is an interface to support dynamic dispatch.
type IParamContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetPid returns the pid token.
	GetPid() antlr.Token

	// SetPid sets the pid token.
	SetPid(antlr.Token)

	// GetT returns the t rule contexts.
	GetT() ITypeContext

	// SetT sets the t rule contexts.
	SetT(ITypeContext)

	// Getter signatures
	COLON() antlr.TerminalNode
	IDENTIFIER() antlr.TerminalNode
	Type_() ITypeContext

	// IsParamContext differentiates from other interfaces.
	IsParamContext()
}

type ParamContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	pid    antlr.Token
	t      ITypeContext
}

func NewEmptyParamContext() *ParamContext {
	var p = new(ParamContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_param
	return p
}

func InitEmptyParamContext(p *ParamContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_param
}

func (*ParamContext) IsParamContext() {}

func NewParamContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ParamContext {
	var p = new(ParamContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_param

	return p
}

func (s *ParamContext) GetParser() antlr.Parser { return s.parser }

func (s *ParamContext) GetPid() antlr.Token { return s.pid }

func (s *ParamContext) SetPid(v antlr.Token) { s.pid = v }

func (s *ParamContext) GetT() ITypeContext { return s.t }

func (s *ParamContext) SetT(v ITypeContext) { s.t = v }

func (s *ParamContext) COLON() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOLON, 0)
}

func (s *ParamContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, 0)
}

func (s *ParamContext) Type_() ITypeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeContext)
}

func (s *ParamContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ParamContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ParamContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterParam(s)
	}
}

func (s *ParamContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitParam(s)
	}
}

func (s *ParamContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitParam(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Param() (localctx IParamContext) {
	localctx = NewParamContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, CommandsParserRULE_param)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(127)

		var _m = p.Match(CommandsParserIDENTIFIER)

		localctx.(*ParamContext).pid = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(128)
		p.Match(CommandsParserCOLON)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(129)

		var _x = p.Type_()

		localctx.(*ParamContext).t = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IDeleteContext is an interface to support dynamic dispatch.
type IDeleteContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetVar_ returns the var_ rule contexts.
	GetVar_() IVarDeclContext

	// GetFn returns the fn rule contexts.
	GetFn() IFnDeclContext

	// SetVar_ sets the var_ rule contexts.
	SetVar_(IVarDeclContext)

	// SetFn sets the fn rule contexts.
	SetFn(IFnDeclContext)

	// Getter signatures
	VarDecl() IVarDeclContext
	FnDecl() IFnDeclContext

	// IsDeleteContext differentiates from other interfaces.
	IsDeleteContext()
}

type DeleteContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	var_   IVarDeclContext
	fn     IFnDeclContext
}

func NewEmptyDeleteContext() *DeleteContext {
	var p = new(DeleteContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_delete
	return p
}

func InitEmptyDeleteContext(p *DeleteContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_delete
}

func (*DeleteContext) IsDeleteContext() {}

func NewDeleteContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *DeleteContext {
	var p = new(DeleteContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_delete

	return p
}

func (s *DeleteContext) GetParser() antlr.Parser { return s.parser }

func (s *DeleteContext) GetVar_() IVarDeclContext { return s.var_ }

func (s *DeleteContext) GetFn() IFnDeclContext { return s.fn }

func (s *DeleteContext) SetVar_(v IVarDeclContext) { s.var_ = v }

func (s *DeleteContext) SetFn(v IFnDeclContext) { s.fn = v }

func (s *DeleteContext) VarDecl() IVarDeclContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IVarDeclContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IVarDeclContext)
}

func (s *DeleteContext) FnDecl() IFnDeclContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFnDeclContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFnDeclContext)
}

func (s *DeleteContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DeleteContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *DeleteContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterDelete(s)
	}
}

func (s *DeleteContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitDelete(s)
	}
}

func (s *DeleteContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitDelete(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Delete_() (localctx IDeleteContext) {
	localctx = NewDeleteContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 16, CommandsParserRULE_delete)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(131)
		p.Match(CommandsParserT__4)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(134)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 6, p.GetParserRuleContext()) {
	case 1:
		{
			p.SetState(132)

			var _x = p.VarDecl()

			localctx.(*DeleteContext).var_ = _x
		}

	case 2:
		{
			p.SetState(133)

			var _x = p.FnDecl()

			localctx.(*DeleteContext).fn = _x
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ISimpleContext is an interface to support dynamic dispatch.
type ISimpleContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetCmd returns the cmd token.
	GetCmd() antlr.Token

	// Get_FLAG returns the _FLAG token.
	Get_FLAG() antlr.Token

	// Get_STRING returns the _STRING token.
	Get_STRING() antlr.Token

	// SetCmd sets the cmd token.
	SetCmd(antlr.Token)

	// Set_FLAG sets the _FLAG token.
	Set_FLAG(antlr.Token)

	// Set_STRING sets the _STRING token.
	Set_STRING(antlr.Token)

	// GetArgs returns the args token list.
	GetArgs() []antlr.Token

	// SetArgs sets the args token list.
	SetArgs([]antlr.Token)

	// Getter signatures
	COMMAND() antlr.TerminalNode
	AllFLAG() []antlr.TerminalNode
	FLAG(i int) antlr.TerminalNode
	AllSTRING() []antlr.TerminalNode
	STRING(i int) antlr.TerminalNode

	// IsSimpleContext differentiates from other interfaces.
	IsSimpleContext()
}

type SimpleContext struct {
	antlr.BaseParserRuleContext
	parser  antlr.Parser
	cmd     antlr.Token
	_FLAG   antlr.Token
	args    []antlr.Token
	_STRING antlr.Token
}

func NewEmptySimpleContext() *SimpleContext {
	var p = new(SimpleContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_simple
	return p
}

func InitEmptySimpleContext(p *SimpleContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_simple
}

func (*SimpleContext) IsSimpleContext() {}

func NewSimpleContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *SimpleContext {
	var p = new(SimpleContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_simple

	return p
}

func (s *SimpleContext) GetParser() antlr.Parser { return s.parser }

func (s *SimpleContext) GetCmd() antlr.Token { return s.cmd }

func (s *SimpleContext) Get_FLAG() antlr.Token { return s._FLAG }

func (s *SimpleContext) Get_STRING() antlr.Token { return s._STRING }

func (s *SimpleContext) SetCmd(v antlr.Token) { s.cmd = v }

func (s *SimpleContext) Set_FLAG(v antlr.Token) { s._FLAG = v }

func (s *SimpleContext) Set_STRING(v antlr.Token) { s._STRING = v }

func (s *SimpleContext) GetArgs() []antlr.Token { return s.args }

func (s *SimpleContext) SetArgs(v []antlr.Token) { s.args = v }

func (s *SimpleContext) COMMAND() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMAND, 0)
}

func (s *SimpleContext) AllFLAG() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserFLAG)
}

func (s *SimpleContext) FLAG(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserFLAG, i)
}

func (s *SimpleContext) AllSTRING() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserSTRING)
}

func (s *SimpleContext) STRING(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserSTRING, i)
}

func (s *SimpleContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SimpleContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *SimpleContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterSimple(s)
	}
}

func (s *SimpleContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitSimple(s)
	}
}

func (s *SimpleContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitSimple(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Simple() (localctx ISimpleContext) {
	localctx = NewSimpleContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 18, CommandsParserRULE_simple)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(136)

		var _m = p.Match(CommandsParserCOMMAND)

		localctx.(*SimpleContext).cmd = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(141)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == CommandsParserFLAG || _la == CommandsParserSTRING {
		p.SetState(139)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}

		switch p.GetTokenStream().LA(1) {
		case CommandsParserFLAG:
			{
				p.SetState(137)

				var _m = p.Match(CommandsParserFLAG)

				localctx.(*SimpleContext)._FLAG = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*SimpleContext).args = append(localctx.(*SimpleContext).args, localctx.(*SimpleContext)._FLAG)

		case CommandsParserSTRING:
			{
				p.SetState(138)

				var _m = p.Match(CommandsParserSTRING)

				localctx.(*SimpleContext)._STRING = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*SimpleContext).args = append(localctx.(*SimpleContext).args, localctx.(*SimpleContext)._STRING)

		default:
			p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
			goto errorExit
		}

		p.SetState(143)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IEmptyContext is an interface to support dynamic dispatch.
type IEmptyContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsEmptyContext differentiates from other interfaces.
	IsEmptyContext()
}

type EmptyContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEmptyContext() *EmptyContext {
	var p = new(EmptyContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_empty
	return p
}

func InitEmptyEmptyContext(p *EmptyContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_empty
}

func (*EmptyContext) IsEmptyContext() {}

func NewEmptyContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EmptyContext {
	var p = new(EmptyContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_empty

	return p
}

func (s *EmptyContext) GetParser() antlr.Parser { return s.parser }
func (s *EmptyContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EmptyContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *EmptyContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterEmpty(s)
	}
}

func (s *EmptyContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitEmpty(s)
	}
}

func (s *EmptyContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitEmpty(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Empty() (localctx IEmptyContext) {
	localctx = NewEmptyContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 20, CommandsParserRULE_empty)
	p.EnterOuterAlt(localctx, 1)

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ICompileContext is an interface to support dynamic dispatch.
type ICompileContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetE returns the e rule contexts.
	GetE() IExprContext

	// SetE sets the e rule contexts.
	SetE(IExprContext)

	// Getter signatures
	Expr() IExprContext

	// IsCompileContext differentiates from other interfaces.
	IsCompileContext()
}

type CompileContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	e      IExprContext
}

func NewEmptyCompileContext() *CompileContext {
	var p = new(CompileContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_compile
	return p
}

func InitEmptyCompileContext(p *CompileContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_compile
}

func (*CompileContext) IsCompileContext() {}

func NewCompileContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CompileContext {
	var p = new(CompileContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_compile

	return p
}

func (s *CompileContext) GetParser() antlr.Parser { return s.parser }

func (s *CompileContext) GetE() IExprContext { return s.e }

func (s *CompileContext) SetE(v IExprContext) { s.e = v }

func (s *CompileContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *CompileContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CompileContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *CompileContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterCompile(s)
	}
}

func (s *CompileContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitCompile(s)
	}
}

func (s *CompileContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitCompile(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Compile() (localctx ICompileContext) {
	localctx = NewCompileContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 22, CommandsParserRULE_compile)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(146)
		p.Match(CommandsParserT__5)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(147)

		var _x = p.Expr()

		localctx.(*CompileContext).e = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IParseContext is an interface to support dynamic dispatch.
type IParseContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetE returns the e rule contexts.
	GetE() IExprContext

	// SetE sets the e rule contexts.
	SetE(IExprContext)

	// Getter signatures
	Expr() IExprContext

	// IsParseContext differentiates from other interfaces.
	IsParseContext()
}

type ParseContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	e      IExprContext
}

func NewEmptyParseContext() *ParseContext {
	var p = new(ParseContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_parse
	return p
}

func InitEmptyParseContext(p *ParseContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_parse
}

func (*ParseContext) IsParseContext() {}

func NewParseContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ParseContext {
	var p = new(ParseContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_parse

	return p
}

func (s *ParseContext) GetParser() antlr.Parser { return s.parser }

func (s *ParseContext) GetE() IExprContext { return s.e }

func (s *ParseContext) SetE(v IExprContext) { s.e = v }

func (s *ParseContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *ParseContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ParseContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ParseContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterParse(s)
	}
}

func (s *ParseContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitParse(s)
	}
}

func (s *ParseContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitParse(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Parse() (localctx IParseContext) {
	localctx = NewParseContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 24, CommandsParserRULE_parse)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(149)
		p.Match(CommandsParserT__6)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(150)

		var _x = p.Expr()

		localctx.(*ParseContext).e = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IExprCmdContext is an interface to support dynamic dispatch.
type IExprCmdContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Get_FLAG returns the _FLAG token.
	Get_FLAG() antlr.Token

	// Set_FLAG sets the _FLAG token.
	Set_FLAG(antlr.Token)

	// GetFlags returns the flags token list.
	GetFlags() []antlr.Token

	// SetFlags sets the flags token list.
	SetFlags([]antlr.Token)

	// GetE returns the e rule contexts.
	GetE() IExprContext

	// SetE sets the e rule contexts.
	SetE(IExprContext)

	// Getter signatures
	Expr() IExprContext
	AllFLAG() []antlr.TerminalNode
	FLAG(i int) antlr.TerminalNode

	// IsExprCmdContext differentiates from other interfaces.
	IsExprCmdContext()
}

type ExprCmdContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	_FLAG  antlr.Token
	flags  []antlr.Token
	e      IExprContext
}

func NewEmptyExprCmdContext() *ExprCmdContext {
	var p = new(ExprCmdContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_exprCmd
	return p
}

func InitEmptyExprCmdContext(p *ExprCmdContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_exprCmd
}

func (*ExprCmdContext) IsExprCmdContext() {}

func NewExprCmdContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExprCmdContext {
	var p = new(ExprCmdContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_exprCmd

	return p
}

func (s *ExprCmdContext) GetParser() antlr.Parser { return s.parser }

func (s *ExprCmdContext) Get_FLAG() antlr.Token { return s._FLAG }

func (s *ExprCmdContext) Set_FLAG(v antlr.Token) { s._FLAG = v }

func (s *ExprCmdContext) GetFlags() []antlr.Token { return s.flags }

func (s *ExprCmdContext) SetFlags(v []antlr.Token) { s.flags = v }

func (s *ExprCmdContext) GetE() IExprContext { return s.e }

func (s *ExprCmdContext) SetE(v IExprContext) { s.e = v }

func (s *ExprCmdContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *ExprCmdContext) AllFLAG() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserFLAG)
}

func (s *ExprCmdContext) FLAG(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserFLAG, i)
}

func (s *ExprCmdContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExprCmdContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ExprCmdContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterExprCmd(s)
	}
}

func (s *ExprCmdContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitExprCmd(s)
	}
}

func (s *ExprCmdContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitExprCmd(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) ExprCmd() (localctx IExprCmdContext) {
	localctx = NewExprCmdContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 26, CommandsParserRULE_exprCmd)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(163)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserT__7 {
		{
			p.SetState(152)
			p.Match(CommandsParserT__7)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(161)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserFLAG {
			{
				p.SetState(153)

				var _m = p.Match(CommandsParserFLAG)

				localctx.(*ExprCmdContext)._FLAG = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*ExprCmdContext).flags = append(localctx.(*ExprCmdContext).flags, localctx.(*ExprCmdContext)._FLAG)
			p.SetState(157)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)

			for _la == CommandsParserFLAG {
				{
					p.SetState(154)

					var _m = p.Match(CommandsParserFLAG)

					localctx.(*ExprCmdContext)._FLAG = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				localctx.(*ExprCmdContext).flags = append(localctx.(*ExprCmdContext).flags, localctx.(*ExprCmdContext)._FLAG)

				p.SetState(159)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)
			}
			{
				p.SetState(160)
				p.Match(CommandsParserT__8)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}

	}
	{
		p.SetState(165)

		var _x = p.Expr()

		localctx.(*ExprCmdContext).e = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IQualIdContext is an interface to support dynamic dispatch.
type IQualIdContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetLeadingDot returns the leadingDot token.
	GetLeadingDot() antlr.Token

	// GetRid returns the rid token.
	GetRid() antlr.Token

	// Get_IDENTIFIER returns the _IDENTIFIER token.
	Get_IDENTIFIER() antlr.Token

	// SetLeadingDot sets the leadingDot token.
	SetLeadingDot(antlr.Token)

	// SetRid sets the rid token.
	SetRid(antlr.Token)

	// Set_IDENTIFIER sets the _IDENTIFIER token.
	Set_IDENTIFIER(antlr.Token)

	// GetQualifiers returns the qualifiers token list.
	GetQualifiers() []antlr.Token

	// SetQualifiers sets the qualifiers token list.
	SetQualifiers([]antlr.Token)

	// Getter signatures
	AllIDENTIFIER() []antlr.TerminalNode
	IDENTIFIER(i int) antlr.TerminalNode
	AllDOT() []antlr.TerminalNode
	DOT(i int) antlr.TerminalNode

	// IsQualIdContext differentiates from other interfaces.
	IsQualIdContext()
}

type QualIdContext struct {
	antlr.BaseParserRuleContext
	parser      antlr.Parser
	leadingDot  antlr.Token
	rid         antlr.Token
	_IDENTIFIER antlr.Token
	qualifiers  []antlr.Token
}

func NewEmptyQualIdContext() *QualIdContext {
	var p = new(QualIdContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_qualId
	return p
}

func InitEmptyQualIdContext(p *QualIdContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_qualId
}

func (*QualIdContext) IsQualIdContext() {}

func NewQualIdContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *QualIdContext {
	var p = new(QualIdContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_qualId

	return p
}

func (s *QualIdContext) GetParser() antlr.Parser { return s.parser }

func (s *QualIdContext) GetLeadingDot() antlr.Token { return s.leadingDot }

func (s *QualIdContext) GetRid() antlr.Token { return s.rid }

func (s *QualIdContext) Get_IDENTIFIER() antlr.Token { return s._IDENTIFIER }

func (s *QualIdContext) SetLeadingDot(v antlr.Token) { s.leadingDot = v }

func (s *QualIdContext) SetRid(v antlr.Token) { s.rid = v }

func (s *QualIdContext) Set_IDENTIFIER(v antlr.Token) { s._IDENTIFIER = v }

func (s *QualIdContext) GetQualifiers() []antlr.Token { return s.qualifiers }

func (s *QualIdContext) SetQualifiers(v []antlr.Token) { s.qualifiers = v }

func (s *QualIdContext) AllIDENTIFIER() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserIDENTIFIER)
}

func (s *QualIdContext) IDENTIFIER(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, i)
}

func (s *QualIdContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserDOT)
}

func (s *QualIdContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, i)
}

func (s *QualIdContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *QualIdContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *QualIdContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterQualId(s)
	}
}

func (s *QualIdContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitQualId(s)
	}
}

func (s *QualIdContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitQualId(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) QualId() (localctx IQualIdContext) {
	localctx = NewQualIdContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 28, CommandsParserRULE_qualId)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(168)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserDOT {
		{
			p.SetState(167)

			var _m = p.Match(CommandsParserDOT)

			localctx.(*QualIdContext).leadingDot = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(170)

		var _m = p.Match(CommandsParserIDENTIFIER)

		localctx.(*QualIdContext).rid = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(175)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == CommandsParserDOT {
		{
			p.SetState(171)
			p.Match(CommandsParserDOT)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(172)

			var _m = p.Match(CommandsParserIDENTIFIER)

			localctx.(*QualIdContext)._IDENTIFIER = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		localctx.(*QualIdContext).qualifiers = append(localctx.(*QualIdContext).qualifiers, localctx.(*QualIdContext)._IDENTIFIER)

		p.SetState(177)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IStartTypeContext is an interface to support dynamic dispatch.
type IStartTypeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetT returns the t rule contexts.
	GetT() ITypeContext

	// SetT sets the t rule contexts.
	SetT(ITypeContext)

	// Getter signatures
	EOF() antlr.TerminalNode
	Type_() ITypeContext

	// IsStartTypeContext differentiates from other interfaces.
	IsStartTypeContext()
}

type StartTypeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	t      ITypeContext
}

func NewEmptyStartTypeContext() *StartTypeContext {
	var p = new(StartTypeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_startType
	return p
}

func InitEmptyStartTypeContext(p *StartTypeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_startType
}

func (*StartTypeContext) IsStartTypeContext() {}

func NewStartTypeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StartTypeContext {
	var p = new(StartTypeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_startType

	return p
}

func (s *StartTypeContext) GetParser() antlr.Parser { return s.parser }

func (s *StartTypeContext) GetT() ITypeContext { return s.t }

func (s *StartTypeContext) SetT(v ITypeContext) { s.t = v }

func (s *StartTypeContext) EOF() antlr.TerminalNode {
	return s.GetToken(CommandsParserEOF, 0)
}

func (s *StartTypeContext) Type_() ITypeContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeContext)
}

func (s *StartTypeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StartTypeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StartTypeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterStartType(s)
	}
}

func (s *StartTypeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitStartType(s)
	}
}

func (s *StartTypeContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitStartType(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) StartType() (localctx IStartTypeContext) {
	localctx = NewStartTypeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 30, CommandsParserRULE_startType)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(178)

		var _x = p.Type_()

		localctx.(*StartTypeContext).t = _x
	}
	{
		p.SetState(179)
		p.Match(CommandsParserEOF)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ITypeContext is an interface to support dynamic dispatch.
type ITypeContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetParams returns the params rule contexts.
	GetParams() ITypeParamListContext

	// SetParams sets the params rule contexts.
	SetParams(ITypeParamListContext)

	// Getter signatures
	TypeId() ITypeIdContext
	TypeParamList() ITypeParamListContext
	ParamId() IParamIdContext

	// IsTypeContext differentiates from other interfaces.
	IsTypeContext()
}

type TypeContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	params ITypeParamListContext
}

func NewEmptyTypeContext() *TypeContext {
	var p = new(TypeContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_type
	return p
}

func InitEmptyTypeContext(p *TypeContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_type
}

func (*TypeContext) IsTypeContext() {}

func NewTypeContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TypeContext {
	var p = new(TypeContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_type

	return p
}

func (s *TypeContext) GetParser() antlr.Parser { return s.parser }

func (s *TypeContext) GetParams() ITypeParamListContext { return s.params }

func (s *TypeContext) SetParams(v ITypeParamListContext) { s.params = v }

func (s *TypeContext) TypeId() ITypeIdContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeIdContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeIdContext)
}

func (s *TypeContext) TypeParamList() ITypeParamListContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeParamListContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeParamListContext)
}

func (s *TypeContext) ParamId() IParamIdContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IParamIdContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IParamIdContext)
}

func (s *TypeContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TypeContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TypeContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterType(s)
	}
}

func (s *TypeContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitType(s)
	}
}

func (s *TypeContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitType(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Type_() (localctx ITypeContext) {
	localctx = NewTypeContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 32, CommandsParserRULE_type)
	var _la int

	p.SetState(186)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case CommandsParserDOT, CommandsParserNUL, CommandsParserIDENTIFIER:
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(181)
			p.TypeId()
		}
		p.SetState(183)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserLESS {
			{
				p.SetState(182)

				var _x = p.TypeParamList()

				localctx.(*TypeContext).params = _x
			}

		}

	case CommandsParserPARAM_SPECIFIER:
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(185)
			p.ParamId()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ITypeIdContext is an interface to support dynamic dispatch.
type ITypeIdContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetLeadingDot returns the leadingDot token.
	GetLeadingDot() antlr.Token

	// GetId returns the id token.
	GetId() antlr.Token

	// Get_IDENTIFIER returns the _IDENTIFIER token.
	Get_IDENTIFIER() antlr.Token

	// SetLeadingDot sets the leadingDot token.
	SetLeadingDot(antlr.Token)

	// SetId sets the id token.
	SetId(antlr.Token)

	// Set_IDENTIFIER sets the _IDENTIFIER token.
	Set_IDENTIFIER(antlr.Token)

	// GetQualifiers returns the qualifiers token list.
	GetQualifiers() []antlr.Token

	// SetQualifiers sets the qualifiers token list.
	SetQualifiers([]antlr.Token)

	// Getter signatures
	AllIDENTIFIER() []antlr.TerminalNode
	IDENTIFIER(i int) antlr.TerminalNode
	NUL() antlr.TerminalNode
	AllDOT() []antlr.TerminalNode
	DOT(i int) antlr.TerminalNode

	// IsTypeIdContext differentiates from other interfaces.
	IsTypeIdContext()
}

type TypeIdContext struct {
	antlr.BaseParserRuleContext
	parser      antlr.Parser
	leadingDot  antlr.Token
	id          antlr.Token
	_IDENTIFIER antlr.Token
	qualifiers  []antlr.Token
}

func NewEmptyTypeIdContext() *TypeIdContext {
	var p = new(TypeIdContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_typeId
	return p
}

func InitEmptyTypeIdContext(p *TypeIdContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_typeId
}

func (*TypeIdContext) IsTypeIdContext() {}

func NewTypeIdContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TypeIdContext {
	var p = new(TypeIdContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_typeId

	return p
}

func (s *TypeIdContext) GetParser() antlr.Parser { return s.parser }

func (s *TypeIdContext) GetLeadingDot() antlr.Token { return s.leadingDot }

func (s *TypeIdContext) GetId() antlr.Token { return s.id }

func (s *TypeIdContext) Get_IDENTIFIER() antlr.Token { return s._IDENTIFIER }

func (s *TypeIdContext) SetLeadingDot(v antlr.Token) { s.leadingDot = v }

func (s *TypeIdContext) SetId(v antlr.Token) { s.id = v }

func (s *TypeIdContext) Set_IDENTIFIER(v antlr.Token) { s._IDENTIFIER = v }

func (s *TypeIdContext) GetQualifiers() []antlr.Token { return s.qualifiers }

func (s *TypeIdContext) SetQualifiers(v []antlr.Token) { s.qualifiers = v }

func (s *TypeIdContext) AllIDENTIFIER() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserIDENTIFIER)
}

func (s *TypeIdContext) IDENTIFIER(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, i)
}

func (s *TypeIdContext) NUL() antlr.TerminalNode {
	return s.GetToken(CommandsParserNUL, 0)
}

func (s *TypeIdContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserDOT)
}

func (s *TypeIdContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, i)
}

func (s *TypeIdContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TypeIdContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TypeIdContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterTypeId(s)
	}
}

func (s *TypeIdContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitTypeId(s)
	}
}

func (s *TypeIdContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitTypeId(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) TypeId() (localctx ITypeIdContext) {
	localctx = NewTypeIdContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 34, CommandsParserRULE_typeId)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(189)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserDOT {
		{
			p.SetState(188)

			var _m = p.Match(CommandsParserDOT)

			localctx.(*TypeIdContext).leadingDot = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(191)

		var _lt = p.GetTokenStream().LT(1)

		localctx.(*TypeIdContext).id = _lt

		_la = p.GetTokenStream().LA(1)

		if !(_la == CommandsParserNUL || _la == CommandsParserIDENTIFIER) {
			var _ri = p.GetErrorHandler().RecoverInline(p)

			localctx.(*TypeIdContext).id = _ri
		} else {
			p.GetErrorHandler().ReportMatch(p)
			p.Consume()
		}
	}
	p.SetState(196)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == CommandsParserDOT {
		{
			p.SetState(192)
			p.Match(CommandsParserDOT)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(193)

			var _m = p.Match(CommandsParserIDENTIFIER)

			localctx.(*TypeIdContext)._IDENTIFIER = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		localctx.(*TypeIdContext).qualifiers = append(localctx.(*TypeIdContext).qualifiers, localctx.(*TypeIdContext)._IDENTIFIER)

		p.SetState(198)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IParamIdContext is an interface to support dynamic dispatch.
type IParamIdContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	PARAM_SPECIFIER() antlr.TerminalNode

	// IsParamIdContext differentiates from other interfaces.
	IsParamIdContext()
}

type ParamIdContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyParamIdContext() *ParamIdContext {
	var p = new(ParamIdContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_paramId
	return p
}

func InitEmptyParamIdContext(p *ParamIdContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_paramId
}

func (*ParamIdContext) IsParamIdContext() {}

func NewParamIdContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ParamIdContext {
	var p = new(ParamIdContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_paramId

	return p
}

func (s *ParamIdContext) GetParser() antlr.Parser { return s.parser }

func (s *ParamIdContext) PARAM_SPECIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserPARAM_SPECIFIER, 0)
}

func (s *ParamIdContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ParamIdContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ParamIdContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterParamId(s)
	}
}

func (s *ParamIdContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitParamId(s)
	}
}

func (s *ParamIdContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitParamId(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) ParamId() (localctx IParamIdContext) {
	localctx = NewParamIdContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 36, CommandsParserRULE_paramId)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(199)
		p.Match(CommandsParserPARAM_SPECIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ITypeParamListContext is an interface to support dynamic dispatch.
type ITypeParamListContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Get_type returns the _type rule contexts.
	Get_type() ITypeContext

	// Set_type sets the _type rule contexts.
	Set_type(ITypeContext)

	// GetTypes returns the types rule context list.
	GetTypes() []ITypeContext

	// SetTypes sets the types rule context list.
	SetTypes([]ITypeContext)

	// Getter signatures
	LESS() antlr.TerminalNode
	GREATER() antlr.TerminalNode
	AllType_() []ITypeContext
	Type_(i int) ITypeContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsTypeParamListContext differentiates from other interfaces.
	IsTypeParamListContext()
}

type TypeParamListContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	_type  ITypeContext
	types  []ITypeContext
}

func NewEmptyTypeParamListContext() *TypeParamListContext {
	var p = new(TypeParamListContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_typeParamList
	return p
}

func InitEmptyTypeParamListContext(p *TypeParamListContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_typeParamList
}

func (*TypeParamListContext) IsTypeParamListContext() {}

func NewTypeParamListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TypeParamListContext {
	var p = new(TypeParamListContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_typeParamList

	return p
}

func (s *TypeParamListContext) GetParser() antlr.Parser { return s.parser }

func (s *TypeParamListContext) Get_type() ITypeContext { return s._type }

func (s *TypeParamListContext) Set_type(v ITypeContext) { s._type = v }

func (s *TypeParamListContext) GetTypes() []ITypeContext { return s.types }

func (s *TypeParamListContext) SetTypes(v []ITypeContext) { s.types = v }

func (s *TypeParamListContext) LESS() antlr.TerminalNode {
	return s.GetToken(CommandsParserLESS, 0)
}

func (s *TypeParamListContext) GREATER() antlr.TerminalNode {
	return s.GetToken(CommandsParserGREATER, 0)
}

func (s *TypeParamListContext) AllType_() []ITypeContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ITypeContext); ok {
			len++
		}
	}

	tst := make([]ITypeContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ITypeContext); ok {
			tst[i] = t.(ITypeContext)
			i++
		}
	}

	return tst
}

func (s *TypeParamListContext) Type_(i int) ITypeContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITypeContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITypeContext)
}

func (s *TypeParamListContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOMMA)
}

func (s *TypeParamListContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, i)
}

func (s *TypeParamListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TypeParamListContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TypeParamListContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterTypeParamList(s)
	}
}

func (s *TypeParamListContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitTypeParamList(s)
	}
}

func (s *TypeParamListContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitTypeParamList(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) TypeParamList() (localctx ITypeParamListContext) {
	localctx = NewTypeParamListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 38, CommandsParserRULE_typeParamList)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(201)
		p.Match(CommandsParserLESS)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	p.SetState(210)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&1130299027111936) != 0 {
		{
			p.SetState(202)

			var _x = p.Type_()

			localctx.(*TypeParamListContext)._type = _x
		}
		localctx.(*TypeParamListContext).types = append(localctx.(*TypeParamListContext).types, localctx.(*TypeParamListContext)._type)
		p.SetState(207)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == CommandsParserCOMMA {
			{
				p.SetState(203)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(204)

				var _x = p.Type_()

				localctx.(*TypeParamListContext)._type = _x
			}
			localctx.(*TypeParamListContext).types = append(localctx.(*TypeParamListContext).types, localctx.(*TypeParamListContext)._type)

			p.SetState(209)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	}
	{
		p.SetState(212)
		p.Match(CommandsParserGREATER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IStartContext is an interface to support dynamic dispatch.
type IStartContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetE returns the e rule contexts.
	GetE() IExprContext

	// SetE sets the e rule contexts.
	SetE(IExprContext)

	// Getter signatures
	EOF() antlr.TerminalNode
	Expr() IExprContext

	// IsStartContext differentiates from other interfaces.
	IsStartContext()
}

type StartContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	e      IExprContext
}

func NewEmptyStartContext() *StartContext {
	var p = new(StartContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_start
	return p
}

func InitEmptyStartContext(p *StartContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_start
}

func (*StartContext) IsStartContext() {}

func NewStartContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StartContext {
	var p = new(StartContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_start

	return p
}

func (s *StartContext) GetParser() antlr.Parser { return s.parser }

func (s *StartContext) GetE() IExprContext { return s.e }

func (s *StartContext) SetE(v IExprContext) { s.e = v }

func (s *StartContext) EOF() antlr.TerminalNode {
	return s.GetToken(CommandsParserEOF, 0)
}

func (s *StartContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *StartContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StartContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StartContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterStart(s)
	}
}

func (s *StartContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitStart(s)
	}
}

func (s *StartContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitStart(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Start_() (localctx IStartContext) {
	localctx = NewStartContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 40, CommandsParserRULE_start)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(214)

		var _x = p.Expr()

		localctx.(*StartContext).e = _x
	}
	{
		p.SetState(215)
		p.Match(CommandsParserEOF)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IExprContext is an interface to support dynamic dispatch.
type IExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetOp returns the op token.
	GetOp() antlr.Token

	// SetOp sets the op token.
	SetOp(antlr.Token)

	// GetE returns the e rule contexts.
	GetE() IConditionalOrContext

	// GetE1 returns the e1 rule contexts.
	GetE1() IConditionalOrContext

	// GetE2 returns the e2 rule contexts.
	GetE2() IExprContext

	// SetE sets the e rule contexts.
	SetE(IConditionalOrContext)

	// SetE1 sets the e1 rule contexts.
	SetE1(IConditionalOrContext)

	// SetE2 sets the e2 rule contexts.
	SetE2(IExprContext)

	// Getter signatures
	AllConditionalOr() []IConditionalOrContext
	ConditionalOr(i int) IConditionalOrContext
	COLON() antlr.TerminalNode
	QUESTIONMARK() antlr.TerminalNode
	Expr() IExprContext

	// IsExprContext differentiates from other interfaces.
	IsExprContext()
}

type ExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	e      IConditionalOrContext
	op     antlr.Token
	e1     IConditionalOrContext
	e2     IExprContext
}

func NewEmptyExprContext() *ExprContext {
	var p = new(ExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_expr
	return p
}

func InitEmptyExprContext(p *ExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_expr
}

func (*ExprContext) IsExprContext() {}

func NewExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExprContext {
	var p = new(ExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_expr

	return p
}

func (s *ExprContext) GetParser() antlr.Parser { return s.parser }

func (s *ExprContext) GetOp() antlr.Token { return s.op }

func (s *ExprContext) SetOp(v antlr.Token) { s.op = v }

func (s *ExprContext) GetE() IConditionalOrContext { return s.e }

func (s *ExprContext) GetE1() IConditionalOrContext { return s.e1 }

func (s *ExprContext) GetE2() IExprContext { return s.e2 }

func (s *ExprContext) SetE(v IConditionalOrContext) { s.e = v }

func (s *ExprContext) SetE1(v IConditionalOrContext) { s.e1 = v }

func (s *ExprContext) SetE2(v IExprContext) { s.e2 = v }

func (s *ExprContext) AllConditionalOr() []IConditionalOrContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IConditionalOrContext); ok {
			len++
		}
	}

	tst := make([]IConditionalOrContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IConditionalOrContext); ok {
			tst[i] = t.(IConditionalOrContext)
			i++
		}
	}

	return tst
}

func (s *ExprContext) ConditionalOr(i int) IConditionalOrContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConditionalOrContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConditionalOrContext)
}

func (s *ExprContext) COLON() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOLON, 0)
}

func (s *ExprContext) QUESTIONMARK() antlr.TerminalNode {
	return s.GetToken(CommandsParserQUESTIONMARK, 0)
}

func (s *ExprContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *ExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterExpr(s)
	}
}

func (s *ExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitExpr(s)
	}
}

func (s *ExprContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitExpr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Expr() (localctx IExprContext) {
	localctx = NewExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 42, CommandsParserRULE_expr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(217)

		var _x = p.ConditionalOr()

		localctx.(*ExprContext).e = _x
	}
	p.SetState(223)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserQUESTIONMARK {
		{
			p.SetState(218)

			var _m = p.Match(CommandsParserQUESTIONMARK)

			localctx.(*ExprContext).op = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(219)

			var _x = p.ConditionalOr()

			localctx.(*ExprContext).e1 = _x
		}
		{
			p.SetState(220)
			p.Match(CommandsParserCOLON)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(221)

			var _x = p.Expr()

			localctx.(*ExprContext).e2 = _x
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IConditionalOrContext is an interface to support dynamic dispatch.
type IConditionalOrContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetS23 returns the s23 token.
	GetS23() antlr.Token

	// SetS23 sets the s23 token.
	SetS23(antlr.Token)

	// GetOps returns the ops token list.
	GetOps() []antlr.Token

	// SetOps sets the ops token list.
	SetOps([]antlr.Token)

	// GetE returns the e rule contexts.
	GetE() IConditionalAndContext

	// Get_conditionalAnd returns the _conditionalAnd rule contexts.
	Get_conditionalAnd() IConditionalAndContext

	// SetE sets the e rule contexts.
	SetE(IConditionalAndContext)

	// Set_conditionalAnd sets the _conditionalAnd rule contexts.
	Set_conditionalAnd(IConditionalAndContext)

	// GetE1 returns the e1 rule context list.
	GetE1() []IConditionalAndContext

	// SetE1 sets the e1 rule context list.
	SetE1([]IConditionalAndContext)

	// Getter signatures
	AllConditionalAnd() []IConditionalAndContext
	ConditionalAnd(i int) IConditionalAndContext
	AllLOGICAL_OR() []antlr.TerminalNode
	LOGICAL_OR(i int) antlr.TerminalNode

	// IsConditionalOrContext differentiates from other interfaces.
	IsConditionalOrContext()
}

type ConditionalOrContext struct {
	antlr.BaseParserRuleContext
	parser          antlr.Parser
	e               IConditionalAndContext
	s23             antlr.Token
	ops             []antlr.Token
	_conditionalAnd IConditionalAndContext
	e1              []IConditionalAndContext
}

func NewEmptyConditionalOrContext() *ConditionalOrContext {
	var p = new(ConditionalOrContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_conditionalOr
	return p
}

func InitEmptyConditionalOrContext(p *ConditionalOrContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_conditionalOr
}

func (*ConditionalOrContext) IsConditionalOrContext() {}

func NewConditionalOrContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ConditionalOrContext {
	var p = new(ConditionalOrContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_conditionalOr

	return p
}

func (s *ConditionalOrContext) GetParser() antlr.Parser { return s.parser }

func (s *ConditionalOrContext) GetS23() antlr.Token { return s.s23 }

func (s *ConditionalOrContext) SetS23(v antlr.Token) { s.s23 = v }

func (s *ConditionalOrContext) GetOps() []antlr.Token { return s.ops }

func (s *ConditionalOrContext) SetOps(v []antlr.Token) { s.ops = v }

func (s *ConditionalOrContext) GetE() IConditionalAndContext { return s.e }

func (s *ConditionalOrContext) Get_conditionalAnd() IConditionalAndContext { return s._conditionalAnd }

func (s *ConditionalOrContext) SetE(v IConditionalAndContext) { s.e = v }

func (s *ConditionalOrContext) Set_conditionalAnd(v IConditionalAndContext) { s._conditionalAnd = v }

func (s *ConditionalOrContext) GetE1() []IConditionalAndContext { return s.e1 }

func (s *ConditionalOrContext) SetE1(v []IConditionalAndContext) { s.e1 = v }

func (s *ConditionalOrContext) AllConditionalAnd() []IConditionalAndContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IConditionalAndContext); ok {
			len++
		}
	}

	tst := make([]IConditionalAndContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IConditionalAndContext); ok {
			tst[i] = t.(IConditionalAndContext)
			i++
		}
	}

	return tst
}

func (s *ConditionalOrContext) ConditionalAnd(i int) IConditionalAndContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConditionalAndContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConditionalAndContext)
}

func (s *ConditionalOrContext) AllLOGICAL_OR() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserLOGICAL_OR)
}

func (s *ConditionalOrContext) LOGICAL_OR(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserLOGICAL_OR, i)
}

func (s *ConditionalOrContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConditionalOrContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ConditionalOrContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterConditionalOr(s)
	}
}

func (s *ConditionalOrContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitConditionalOr(s)
	}
}

func (s *ConditionalOrContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitConditionalOr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) ConditionalOr() (localctx IConditionalOrContext) {
	localctx = NewConditionalOrContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 44, CommandsParserRULE_conditionalOr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(225)

		var _x = p.ConditionalAnd()

		localctx.(*ConditionalOrContext).e = _x
	}
	p.SetState(230)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == CommandsParserLOGICAL_OR {
		{
			p.SetState(226)

			var _m = p.Match(CommandsParserLOGICAL_OR)

			localctx.(*ConditionalOrContext).s23 = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		localctx.(*ConditionalOrContext).ops = append(localctx.(*ConditionalOrContext).ops, localctx.(*ConditionalOrContext).s23)
		{
			p.SetState(227)

			var _x = p.ConditionalAnd()

			localctx.(*ConditionalOrContext)._conditionalAnd = _x
		}
		localctx.(*ConditionalOrContext).e1 = append(localctx.(*ConditionalOrContext).e1, localctx.(*ConditionalOrContext)._conditionalAnd)

		p.SetState(232)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IConditionalAndContext is an interface to support dynamic dispatch.
type IConditionalAndContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetS22 returns the s22 token.
	GetS22() antlr.Token

	// SetS22 sets the s22 token.
	SetS22(antlr.Token)

	// GetOps returns the ops token list.
	GetOps() []antlr.Token

	// SetOps sets the ops token list.
	SetOps([]antlr.Token)

	// GetE returns the e rule contexts.
	GetE() IRelationContext

	// Get_relation returns the _relation rule contexts.
	Get_relation() IRelationContext

	// SetE sets the e rule contexts.
	SetE(IRelationContext)

	// Set_relation sets the _relation rule contexts.
	Set_relation(IRelationContext)

	// GetE1 returns the e1 rule context list.
	GetE1() []IRelationContext

	// SetE1 sets the e1 rule context list.
	SetE1([]IRelationContext)

	// Getter signatures
	AllRelation() []IRelationContext
	Relation(i int) IRelationContext
	AllLOGICAL_AND() []antlr.TerminalNode
	LOGICAL_AND(i int) antlr.TerminalNode

	// IsConditionalAndContext differentiates from other interfaces.
	IsConditionalAndContext()
}

type ConditionalAndContext struct {
	antlr.BaseParserRuleContext
	parser    antlr.Parser
	e         IRelationContext
	s22       antlr.Token
	ops       []antlr.Token
	_relation IRelationContext
	e1        []IRelationContext
}

func NewEmptyConditionalAndContext() *ConditionalAndContext {
	var p = new(ConditionalAndContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_conditionalAnd
	return p
}

func InitEmptyConditionalAndContext(p *ConditionalAndContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_conditionalAnd
}

func (*ConditionalAndContext) IsConditionalAndContext() {}

func NewConditionalAndContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ConditionalAndContext {
	var p = new(ConditionalAndContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_conditionalAnd

	return p
}

func (s *ConditionalAndContext) GetParser() antlr.Parser { return s.parser }

func (s *ConditionalAndContext) GetS22() antlr.Token { return s.s22 }

func (s *ConditionalAndContext) SetS22(v antlr.Token) { s.s22 = v }

func (s *ConditionalAndContext) GetOps() []antlr.Token { return s.ops }

func (s *ConditionalAndContext) SetOps(v []antlr.Token) { s.ops = v }

func (s *ConditionalAndContext) GetE() IRelationContext { return s.e }

func (s *ConditionalAndContext) Get_relation() IRelationContext { return s._relation }

func (s *ConditionalAndContext) SetE(v IRelationContext) { s.e = v }

func (s *ConditionalAndContext) Set_relation(v IRelationContext) { s._relation = v }

func (s *ConditionalAndContext) GetE1() []IRelationContext { return s.e1 }

func (s *ConditionalAndContext) SetE1(v []IRelationContext) { s.e1 = v }

func (s *ConditionalAndContext) AllRelation() []IRelationContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IRelationContext); ok {
			len++
		}
	}

	tst := make([]IRelationContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IRelationContext); ok {
			tst[i] = t.(IRelationContext)
			i++
		}
	}

	return tst
}

func (s *ConditionalAndContext) Relation(i int) IRelationContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRelationContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRelationContext)
}

func (s *ConditionalAndContext) AllLOGICAL_AND() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserLOGICAL_AND)
}

func (s *ConditionalAndContext) LOGICAL_AND(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserLOGICAL_AND, i)
}

func (s *ConditionalAndContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConditionalAndContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ConditionalAndContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterConditionalAnd(s)
	}
}

func (s *ConditionalAndContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitConditionalAnd(s)
	}
}

func (s *ConditionalAndContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitConditionalAnd(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) ConditionalAnd() (localctx IConditionalAndContext) {
	localctx = NewConditionalAndContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 46, CommandsParserRULE_conditionalAnd)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(233)

		var _x = p.relation(0)

		localctx.(*ConditionalAndContext).e = _x
	}
	p.SetState(238)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == CommandsParserLOGICAL_AND {
		{
			p.SetState(234)

			var _m = p.Match(CommandsParserLOGICAL_AND)

			localctx.(*ConditionalAndContext).s22 = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		localctx.(*ConditionalAndContext).ops = append(localctx.(*ConditionalAndContext).ops, localctx.(*ConditionalAndContext).s22)
		{
			p.SetState(235)

			var _x = p.relation(0)

			localctx.(*ConditionalAndContext)._relation = _x
		}
		localctx.(*ConditionalAndContext).e1 = append(localctx.(*ConditionalAndContext).e1, localctx.(*ConditionalAndContext)._relation)

		p.SetState(240)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IRelationContext is an interface to support dynamic dispatch.
type IRelationContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetOp returns the op token.
	GetOp() antlr.Token

	// SetOp sets the op token.
	SetOp(antlr.Token)

	// Getter signatures
	Calc() ICalcContext
	AllRelation() []IRelationContext
	Relation(i int) IRelationContext
	LESS() antlr.TerminalNode
	LESS_EQUALS() antlr.TerminalNode
	GREATER_EQUALS() antlr.TerminalNode
	GREATER() antlr.TerminalNode
	EQUALS() antlr.TerminalNode
	NOT_EQUALS() antlr.TerminalNode
	IN() antlr.TerminalNode

	// IsRelationContext differentiates from other interfaces.
	IsRelationContext()
}

type RelationContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	op     antlr.Token
}

func NewEmptyRelationContext() *RelationContext {
	var p = new(RelationContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_relation
	return p
}

func InitEmptyRelationContext(p *RelationContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_relation
}

func (*RelationContext) IsRelationContext() {}

func NewRelationContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *RelationContext {
	var p = new(RelationContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_relation

	return p
}

func (s *RelationContext) GetParser() antlr.Parser { return s.parser }

func (s *RelationContext) GetOp() antlr.Token { return s.op }

func (s *RelationContext) SetOp(v antlr.Token) { s.op = v }

func (s *RelationContext) Calc() ICalcContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICalcContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ICalcContext)
}

func (s *RelationContext) AllRelation() []IRelationContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IRelationContext); ok {
			len++
		}
	}

	tst := make([]IRelationContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IRelationContext); ok {
			tst[i] = t.(IRelationContext)
			i++
		}
	}

	return tst
}

func (s *RelationContext) Relation(i int) IRelationContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IRelationContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IRelationContext)
}

func (s *RelationContext) LESS() antlr.TerminalNode {
	return s.GetToken(CommandsParserLESS, 0)
}

func (s *RelationContext) LESS_EQUALS() antlr.TerminalNode {
	return s.GetToken(CommandsParserLESS_EQUALS, 0)
}

func (s *RelationContext) GREATER_EQUALS() antlr.TerminalNode {
	return s.GetToken(CommandsParserGREATER_EQUALS, 0)
}

func (s *RelationContext) GREATER() antlr.TerminalNode {
	return s.GetToken(CommandsParserGREATER, 0)
}

func (s *RelationContext) EQUALS() antlr.TerminalNode {
	return s.GetToken(CommandsParserEQUALS, 0)
}

func (s *RelationContext) NOT_EQUALS() antlr.TerminalNode {
	return s.GetToken(CommandsParserNOT_EQUALS, 0)
}

func (s *RelationContext) IN() antlr.TerminalNode {
	return s.GetToken(CommandsParserIN, 0)
}

func (s *RelationContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *RelationContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *RelationContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterRelation(s)
	}
}

func (s *RelationContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitRelation(s)
	}
}

func (s *RelationContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitRelation(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Relation() (localctx IRelationContext) {
	return p.relation(0)
}

func (p *CommandsParser) relation(_p int) (localctx IRelationContext) {
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()

	_parentState := p.GetState()
	localctx = NewRelationContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IRelationContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 48
	p.EnterRecursionRule(localctx, 48, CommandsParserRULE_relation, _p)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(242)
		p.calc(0)
	}

	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(249)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 23, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			localctx = NewRelationContext(p, _parentctx, _parentState)
			p.PushNewRecursionContext(localctx, _startState, CommandsParserRULE_relation)
			p.SetState(244)

			if !(p.Precpred(p.GetParserRuleContext(), 1)) {
				p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 1)", ""))
				goto errorExit
			}
			{
				p.SetState(245)

				var _lt = p.GetTokenStream().LT(1)

				localctx.(*RelationContext).op = _lt

				_la = p.GetTokenStream().LA(1)

				if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&4161536) != 0) {
					var _ri = p.GetErrorHandler().RecoverInline(p)

					localctx.(*RelationContext).op = _ri
				} else {
					p.GetErrorHandler().ReportMatch(p)
					p.Consume()
				}
			}
			{
				p.SetState(246)
				p.relation(2)
			}

		}
		p.SetState(251)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 23, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ICalcContext is an interface to support dynamic dispatch.
type ICalcContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetOp returns the op token.
	GetOp() antlr.Token

	// SetOp sets the op token.
	SetOp(antlr.Token)

	// Getter signatures
	Unary() IUnaryContext
	AllCalc() []ICalcContext
	Calc(i int) ICalcContext
	STAR() antlr.TerminalNode
	SLASH() antlr.TerminalNode
	PERCENT() antlr.TerminalNode
	PLUS() antlr.TerminalNode
	MINUS() antlr.TerminalNode

	// IsCalcContext differentiates from other interfaces.
	IsCalcContext()
}

type CalcContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	op     antlr.Token
}

func NewEmptyCalcContext() *CalcContext {
	var p = new(CalcContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_calc
	return p
}

func InitEmptyCalcContext(p *CalcContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_calc
}

func (*CalcContext) IsCalcContext() {}

func NewCalcContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *CalcContext {
	var p = new(CalcContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_calc

	return p
}

func (s *CalcContext) GetParser() antlr.Parser { return s.parser }

func (s *CalcContext) GetOp() antlr.Token { return s.op }

func (s *CalcContext) SetOp(v antlr.Token) { s.op = v }

func (s *CalcContext) Unary() IUnaryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUnaryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUnaryContext)
}

func (s *CalcContext) AllCalc() []ICalcContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(ICalcContext); ok {
			len++
		}
	}

	tst := make([]ICalcContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(ICalcContext); ok {
			tst[i] = t.(ICalcContext)
			i++
		}
	}

	return tst
}

func (s *CalcContext) Calc(i int) ICalcContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ICalcContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(ICalcContext)
}

func (s *CalcContext) STAR() antlr.TerminalNode {
	return s.GetToken(CommandsParserSTAR, 0)
}

func (s *CalcContext) SLASH() antlr.TerminalNode {
	return s.GetToken(CommandsParserSLASH, 0)
}

func (s *CalcContext) PERCENT() antlr.TerminalNode {
	return s.GetToken(CommandsParserPERCENT, 0)
}

func (s *CalcContext) PLUS() antlr.TerminalNode {
	return s.GetToken(CommandsParserPLUS, 0)
}

func (s *CalcContext) MINUS() antlr.TerminalNode {
	return s.GetToken(CommandsParserMINUS, 0)
}

func (s *CalcContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CalcContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *CalcContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterCalc(s)
	}
}

func (s *CalcContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitCalc(s)
	}
}

func (s *CalcContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitCalc(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Calc() (localctx ICalcContext) {
	return p.calc(0)
}

func (p *CommandsParser) calc(_p int) (localctx ICalcContext) {
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()

	_parentState := p.GetState()
	localctx = NewCalcContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx ICalcContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 50
	p.EnterRecursionRule(localctx, 50, CommandsParserRULE_calc, _p)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(253)
		p.Unary()
	}

	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(263)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 25, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(261)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 24, p.GetParserRuleContext()) {
			case 1:
				localctx = NewCalcContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, CommandsParserRULE_calc)
				p.SetState(255)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
					goto errorExit
				}
				{
					p.SetState(256)

					var _lt = p.GetTokenStream().LT(1)

					localctx.(*CalcContext).op = _lt

					_la = p.GetTokenStream().LA(1)

					if !((int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&962072674304) != 0) {
						var _ri = p.GetErrorHandler().RecoverInline(p)

						localctx.(*CalcContext).op = _ri
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(257)
					p.calc(3)
				}

			case 2:
				localctx = NewCalcContext(p, _parentctx, _parentState)
				p.PushNewRecursionContext(localctx, _startState, CommandsParserRULE_calc)
				p.SetState(258)

				if !(p.Precpred(p.GetParserRuleContext(), 1)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 1)", ""))
					goto errorExit
				}
				{
					p.SetState(259)

					var _lt = p.GetTokenStream().LT(1)

					localctx.(*CalcContext).op = _lt

					_la = p.GetTokenStream().LA(1)

					if !(_la == CommandsParserMINUS || _la == CommandsParserPLUS) {
						var _ri = p.GetErrorHandler().RecoverInline(p)

						localctx.(*CalcContext).op = _ri
					} else {
						p.GetErrorHandler().ReportMatch(p)
						p.Consume()
					}
				}
				{
					p.SetState(260)
					p.calc(2)
				}

			case antlr.ATNInvalidAltNumber:
				goto errorExit
			}

		}
		p.SetState(265)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 25, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IUnaryContext is an interface to support dynamic dispatch.
type IUnaryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsUnaryContext differentiates from other interfaces.
	IsUnaryContext()
}

type UnaryContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyUnaryContext() *UnaryContext {
	var p = new(UnaryContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_unary
	return p
}

func InitEmptyUnaryContext(p *UnaryContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_unary
}

func (*UnaryContext) IsUnaryContext() {}

func NewUnaryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UnaryContext {
	var p = new(UnaryContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_unary

	return p
}

func (s *UnaryContext) GetParser() antlr.Parser { return s.parser }

func (s *UnaryContext) CopyAll(ctx *UnaryContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *UnaryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UnaryContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type LogicalNotContext struct {
	UnaryContext
	s33 antlr.Token
	ops []antlr.Token
}

func NewLogicalNotContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *LogicalNotContext {
	var p = new(LogicalNotContext)

	InitEmptyUnaryContext(&p.UnaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*UnaryContext))

	return p
}

func (s *LogicalNotContext) GetS33() antlr.Token { return s.s33 }

func (s *LogicalNotContext) SetS33(v antlr.Token) { s.s33 = v }

func (s *LogicalNotContext) GetOps() []antlr.Token { return s.ops }

func (s *LogicalNotContext) SetOps(v []antlr.Token) { s.ops = v }

func (s *LogicalNotContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LogicalNotContext) Member() IMemberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMemberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMemberContext)
}

func (s *LogicalNotContext) AllEXCLAM() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserEXCLAM)
}

func (s *LogicalNotContext) EXCLAM(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserEXCLAM, i)
}

func (s *LogicalNotContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterLogicalNot(s)
	}
}

func (s *LogicalNotContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitLogicalNot(s)
	}
}

func (s *LogicalNotContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitLogicalNot(s)

	default:
		return t.VisitChildren(s)
	}
}

type MemberExprContext struct {
	UnaryContext
}

func NewMemberExprContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *MemberExprContext {
	var p = new(MemberExprContext)

	InitEmptyUnaryContext(&p.UnaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*UnaryContext))

	return p
}

func (s *MemberExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MemberExprContext) Member() IMemberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMemberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMemberContext)
}

func (s *MemberExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterMemberExpr(s)
	}
}

func (s *MemberExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitMemberExpr(s)
	}
}

func (s *MemberExprContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitMemberExpr(s)

	default:
		return t.VisitChildren(s)
	}
}

type NegateContext struct {
	UnaryContext
	s32 antlr.Token
	ops []antlr.Token
}

func NewNegateContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *NegateContext {
	var p = new(NegateContext)

	InitEmptyUnaryContext(&p.UnaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*UnaryContext))

	return p
}

func (s *NegateContext) GetS32() antlr.Token { return s.s32 }

func (s *NegateContext) SetS32(v antlr.Token) { s.s32 = v }

func (s *NegateContext) GetOps() []antlr.Token { return s.ops }

func (s *NegateContext) SetOps(v []antlr.Token) { s.ops = v }

func (s *NegateContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NegateContext) Member() IMemberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMemberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMemberContext)
}

func (s *NegateContext) AllMINUS() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserMINUS)
}

func (s *NegateContext) MINUS(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserMINUS, i)
}

func (s *NegateContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterNegate(s)
	}
}

func (s *NegateContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitNegate(s)
	}
}

func (s *NegateContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitNegate(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Unary() (localctx IUnaryContext) {
	localctx = NewUnaryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 52, CommandsParserRULE_unary)
	var _la int

	var _alt int

	p.SetState(279)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 28, p.GetParserRuleContext()) {
	case 1:
		localctx = NewMemberExprContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(266)
			p.member(0)
		}

	case 2:
		localctx = NewLogicalNotContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		p.SetState(268)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for ok := true; ok; ok = _la == CommandsParserEXCLAM {
			{
				p.SetState(267)

				var _m = p.Match(CommandsParserEXCLAM)

				localctx.(*LogicalNotContext).s33 = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*LogicalNotContext).ops = append(localctx.(*LogicalNotContext).ops, localctx.(*LogicalNotContext).s33)

			p.SetState(270)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}
		{
			p.SetState(272)
			p.member(0)
		}

	case 3:
		localctx = NewNegateContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		p.SetState(274)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = 1
		for ok := true; ok; ok = _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
			switch _alt {
			case 1:
				{
					p.SetState(273)

					var _m = p.Match(CommandsParserMINUS)

					localctx.(*NegateContext).s32 = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				localctx.(*NegateContext).ops = append(localctx.(*NegateContext).ops, localctx.(*NegateContext).s32)

			default:
				p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
				goto errorExit
			}

			p.SetState(276)
			p.GetErrorHandler().Sync(p)
			_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 27, p.GetParserRuleContext())
			if p.HasError() {
				goto errorExit
			}
		}
		{
			p.SetState(278)
			p.member(0)
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IMemberContext is an interface to support dynamic dispatch.
type IMemberContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsMemberContext differentiates from other interfaces.
	IsMemberContext()
}

type MemberContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyMemberContext() *MemberContext {
	var p = new(MemberContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_member
	return p
}

func InitEmptyMemberContext(p *MemberContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_member
}

func (*MemberContext) IsMemberContext() {}

func NewMemberContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *MemberContext {
	var p = new(MemberContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_member

	return p
}

func (s *MemberContext) GetParser() antlr.Parser { return s.parser }

func (s *MemberContext) CopyAll(ctx *MemberContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *MemberContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MemberContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type MemberCallContext struct {
	MemberContext
	op   antlr.Token
	id   antlr.Token
	open antlr.Token
	args IExprListContext
}

func NewMemberCallContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *MemberCallContext {
	var p = new(MemberCallContext)

	InitEmptyMemberContext(&p.MemberContext)
	p.parser = parser
	p.CopyAll(ctx.(*MemberContext))

	return p
}

func (s *MemberCallContext) GetOp() antlr.Token { return s.op }

func (s *MemberCallContext) GetId() antlr.Token { return s.id }

func (s *MemberCallContext) GetOpen() antlr.Token { return s.open }

func (s *MemberCallContext) SetOp(v antlr.Token) { s.op = v }

func (s *MemberCallContext) SetId(v antlr.Token) { s.id = v }

func (s *MemberCallContext) SetOpen(v antlr.Token) { s.open = v }

func (s *MemberCallContext) GetArgs() IExprListContext { return s.args }

func (s *MemberCallContext) SetArgs(v IExprListContext) { s.args = v }

func (s *MemberCallContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MemberCallContext) Member() IMemberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMemberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMemberContext)
}

func (s *MemberCallContext) RPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserRPAREN, 0)
}

func (s *MemberCallContext) DOT() antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, 0)
}

func (s *MemberCallContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, 0)
}

func (s *MemberCallContext) LPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserLPAREN, 0)
}

func (s *MemberCallContext) ExprList() IExprListContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprListContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprListContext)
}

func (s *MemberCallContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterMemberCall(s)
	}
}

func (s *MemberCallContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitMemberCall(s)
	}
}

func (s *MemberCallContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitMemberCall(s)

	default:
		return t.VisitChildren(s)
	}
}

type SelectContext struct {
	MemberContext
	op  antlr.Token
	opt antlr.Token
	id  IEscapeIdentContext
}

func NewSelectContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *SelectContext {
	var p = new(SelectContext)

	InitEmptyMemberContext(&p.MemberContext)
	p.parser = parser
	p.CopyAll(ctx.(*MemberContext))

	return p
}

func (s *SelectContext) GetOp() antlr.Token { return s.op }

func (s *SelectContext) GetOpt() antlr.Token { return s.opt }

func (s *SelectContext) SetOp(v antlr.Token) { s.op = v }

func (s *SelectContext) SetOpt(v antlr.Token) { s.opt = v }

func (s *SelectContext) GetId() IEscapeIdentContext { return s.id }

func (s *SelectContext) SetId(v IEscapeIdentContext) { s.id = v }

func (s *SelectContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SelectContext) Member() IMemberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMemberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMemberContext)
}

func (s *SelectContext) DOT() antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, 0)
}

func (s *SelectContext) EscapeIdent() IEscapeIdentContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEscapeIdentContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEscapeIdentContext)
}

func (s *SelectContext) QUESTIONMARK() antlr.TerminalNode {
	return s.GetToken(CommandsParserQUESTIONMARK, 0)
}

func (s *SelectContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterSelect(s)
	}
}

func (s *SelectContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitSelect(s)
	}
}

func (s *SelectContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitSelect(s)

	default:
		return t.VisitChildren(s)
	}
}

type PrimaryExprContext struct {
	MemberContext
}

func NewPrimaryExprContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *PrimaryExprContext {
	var p = new(PrimaryExprContext)

	InitEmptyMemberContext(&p.MemberContext)
	p.parser = parser
	p.CopyAll(ctx.(*MemberContext))

	return p
}

func (s *PrimaryExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PrimaryExprContext) Primary() IPrimaryContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IPrimaryContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IPrimaryContext)
}

func (s *PrimaryExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterPrimaryExpr(s)
	}
}

func (s *PrimaryExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitPrimaryExpr(s)
	}
}

func (s *PrimaryExprContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitPrimaryExpr(s)

	default:
		return t.VisitChildren(s)
	}
}

type IndexContext struct {
	MemberContext
	op    antlr.Token
	opt   antlr.Token
	index IExprContext
}

func NewIndexContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *IndexContext {
	var p = new(IndexContext)

	InitEmptyMemberContext(&p.MemberContext)
	p.parser = parser
	p.CopyAll(ctx.(*MemberContext))

	return p
}

func (s *IndexContext) GetOp() antlr.Token { return s.op }

func (s *IndexContext) GetOpt() antlr.Token { return s.opt }

func (s *IndexContext) SetOp(v antlr.Token) { s.op = v }

func (s *IndexContext) SetOpt(v antlr.Token) { s.opt = v }

func (s *IndexContext) GetIndex() IExprContext { return s.index }

func (s *IndexContext) SetIndex(v IExprContext) { s.index = v }

func (s *IndexContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IndexContext) Member() IMemberContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMemberContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMemberContext)
}

func (s *IndexContext) RPRACKET() antlr.TerminalNode {
	return s.GetToken(CommandsParserRPRACKET, 0)
}

func (s *IndexContext) LBRACKET() antlr.TerminalNode {
	return s.GetToken(CommandsParserLBRACKET, 0)
}

func (s *IndexContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *IndexContext) QUESTIONMARK() antlr.TerminalNode {
	return s.GetToken(CommandsParserQUESTIONMARK, 0)
}

func (s *IndexContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterIndex(s)
	}
}

func (s *IndexContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitIndex(s)
	}
}

func (s *IndexContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitIndex(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Member() (localctx IMemberContext) {
	return p.member(0)
}

func (p *CommandsParser) member(_p int) (localctx IMemberContext) {
	var _parentctx antlr.ParserRuleContext = p.GetParserRuleContext()

	_parentState := p.GetState()
	localctx = NewMemberContext(p, p.GetParserRuleContext(), _parentState)
	var _prevctx IMemberContext = localctx
	var _ antlr.ParserRuleContext = _prevctx // TODO: To prevent unused variable warning.
	_startState := 54
	p.EnterRecursionRule(localctx, 54, CommandsParserRULE_member, _p)
	var _la int

	var _alt int

	p.EnterOuterAlt(localctx, 1)
	localctx = NewPrimaryExprContext(p, localctx)
	p.SetParserRuleContext(localctx)
	_prevctx = localctx

	{
		p.SetState(282)
		p.Primary()
	}

	p.GetParserRuleContext().SetStop(p.GetTokenStream().LT(-1))
	p.SetState(308)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 33, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			if p.GetParseListeners() != nil {
				p.TriggerExitRuleEvent()
			}
			_prevctx = localctx
			p.SetState(306)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 32, p.GetParserRuleContext()) {
			case 1:
				localctx = NewSelectContext(p, NewMemberContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CommandsParserRULE_member)
				p.SetState(284)

				if !(p.Precpred(p.GetParserRuleContext(), 3)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 3)", ""))
					goto errorExit
				}
				{
					p.SetState(285)

					var _m = p.Match(CommandsParserDOT)

					localctx.(*SelectContext).op = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				p.SetState(287)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)

				if _la == CommandsParserQUESTIONMARK {
					{
						p.SetState(286)

						var _m = p.Match(CommandsParserQUESTIONMARK)

						localctx.(*SelectContext).opt = _m
						if p.HasError() {
							// Recognition error - abort rule
							goto errorExit
						}
					}

				}
				{
					p.SetState(289)

					var _x = p.EscapeIdent()

					localctx.(*SelectContext).id = _x
				}

			case 2:
				localctx = NewMemberCallContext(p, NewMemberContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CommandsParserRULE_member)
				p.SetState(290)

				if !(p.Precpred(p.GetParserRuleContext(), 2)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 2)", ""))
					goto errorExit
				}
				{
					p.SetState(291)

					var _m = p.Match(CommandsParserDOT)

					localctx.(*MemberCallContext).op = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(292)

					var _m = p.Match(CommandsParserIDENTIFIER)

					localctx.(*MemberCallContext).id = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(293)

					var _m = p.Match(CommandsParserLPAREN)

					localctx.(*MemberCallContext).open = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				p.SetState(295)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)

				if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2224326333956096) != 0 {
					{
						p.SetState(294)

						var _x = p.ExprList()

						localctx.(*MemberCallContext).args = _x
					}

				}
				{
					p.SetState(297)
					p.Match(CommandsParserRPAREN)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}

			case 3:
				localctx = NewIndexContext(p, NewMemberContext(p, _parentctx, _parentState))
				p.PushNewRecursionContext(localctx, _startState, CommandsParserRULE_member)
				p.SetState(298)

				if !(p.Precpred(p.GetParserRuleContext(), 1)) {
					p.SetError(antlr.NewFailedPredicateException(p, "p.Precpred(p.GetParserRuleContext(), 1)", ""))
					goto errorExit
				}
				{
					p.SetState(299)

					var _m = p.Match(CommandsParserLBRACKET)

					localctx.(*IndexContext).op = _m
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				p.SetState(301)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)

				if _la == CommandsParserQUESTIONMARK {
					{
						p.SetState(300)

						var _m = p.Match(CommandsParserQUESTIONMARK)

						localctx.(*IndexContext).opt = _m
						if p.HasError() {
							// Recognition error - abort rule
							goto errorExit
						}
					}

				}
				{
					p.SetState(303)

					var _x = p.Expr()

					localctx.(*IndexContext).index = _x
				}
				{
					p.SetState(304)
					p.Match(CommandsParserRPRACKET)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}

			case antlr.ATNInvalidAltNumber:
				goto errorExit
			}

		}
		p.SetState(310)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 33, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.UnrollRecursionContexts(_parentctx)
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IPrimaryContext is an interface to support dynamic dispatch.
type IPrimaryContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsPrimaryContext differentiates from other interfaces.
	IsPrimaryContext()
}

type PrimaryContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyPrimaryContext() *PrimaryContext {
	var p = new(PrimaryContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_primary
	return p
}

func InitEmptyPrimaryContext(p *PrimaryContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_primary
}

func (*PrimaryContext) IsPrimaryContext() {}

func NewPrimaryContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *PrimaryContext {
	var p = new(PrimaryContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_primary

	return p
}

func (s *PrimaryContext) GetParser() antlr.Parser { return s.parser }

func (s *PrimaryContext) CopyAll(ctx *PrimaryContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *PrimaryContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PrimaryContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type CreateListContext struct {
	PrimaryContext
	op    antlr.Token
	elems IListInitContext
}

func NewCreateListContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *CreateListContext {
	var p = new(CreateListContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *CreateListContext) GetOp() antlr.Token { return s.op }

func (s *CreateListContext) SetOp(v antlr.Token) { s.op = v }

func (s *CreateListContext) GetElems() IListInitContext { return s.elems }

func (s *CreateListContext) SetElems(v IListInitContext) { s.elems = v }

func (s *CreateListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CreateListContext) RPRACKET() antlr.TerminalNode {
	return s.GetToken(CommandsParserRPRACKET, 0)
}

func (s *CreateListContext) LBRACKET() antlr.TerminalNode {
	return s.GetToken(CommandsParserLBRACKET, 0)
}

func (s *CreateListContext) COMMA() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, 0)
}

func (s *CreateListContext) ListInit() IListInitContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IListInitContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IListInitContext)
}

func (s *CreateListContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterCreateList(s)
	}
}

func (s *CreateListContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitCreateList(s)
	}
}

func (s *CreateListContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitCreateList(s)

	default:
		return t.VisitChildren(s)
	}
}

type IdentContext struct {
	PrimaryContext
	leadingDot antlr.Token
	id         antlr.Token
}

func NewIdentContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *IdentContext {
	var p = new(IdentContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *IdentContext) GetLeadingDot() antlr.Token { return s.leadingDot }

func (s *IdentContext) GetId() antlr.Token { return s.id }

func (s *IdentContext) SetLeadingDot(v antlr.Token) { s.leadingDot = v }

func (s *IdentContext) SetId(v antlr.Token) { s.id = v }

func (s *IdentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IdentContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, 0)
}

func (s *IdentContext) DOT() antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, 0)
}

func (s *IdentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterIdent(s)
	}
}

func (s *IdentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitIdent(s)
	}
}

func (s *IdentContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitIdent(s)

	default:
		return t.VisitChildren(s)
	}
}

type CreateStructContext struct {
	PrimaryContext
	op      antlr.Token
	entries IMapInitializerListContext
}

func NewCreateStructContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *CreateStructContext {
	var p = new(CreateStructContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *CreateStructContext) GetOp() antlr.Token { return s.op }

func (s *CreateStructContext) SetOp(v antlr.Token) { s.op = v }

func (s *CreateStructContext) GetEntries() IMapInitializerListContext { return s.entries }

func (s *CreateStructContext) SetEntries(v IMapInitializerListContext) { s.entries = v }

func (s *CreateStructContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CreateStructContext) RBRACE() antlr.TerminalNode {
	return s.GetToken(CommandsParserRBRACE, 0)
}

func (s *CreateStructContext) LBRACE() antlr.TerminalNode {
	return s.GetToken(CommandsParserLBRACE, 0)
}

func (s *CreateStructContext) COMMA() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, 0)
}

func (s *CreateStructContext) MapInitializerList() IMapInitializerListContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IMapInitializerListContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IMapInitializerListContext)
}

func (s *CreateStructContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterCreateStruct(s)
	}
}

func (s *CreateStructContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitCreateStruct(s)
	}
}

func (s *CreateStructContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitCreateStruct(s)

	default:
		return t.VisitChildren(s)
	}
}

type ConstantLiteralContext struct {
	PrimaryContext
}

func NewConstantLiteralContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ConstantLiteralContext {
	var p = new(ConstantLiteralContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *ConstantLiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConstantLiteralContext) Literal() ILiteralContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ILiteralContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ILiteralContext)
}

func (s *ConstantLiteralContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterConstantLiteral(s)
	}
}

func (s *ConstantLiteralContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitConstantLiteral(s)
	}
}

func (s *ConstantLiteralContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitConstantLiteral(s)

	default:
		return t.VisitChildren(s)
	}
}

type NestedContext struct {
	PrimaryContext
	e IExprContext
}

func NewNestedContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *NestedContext {
	var p = new(NestedContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *NestedContext) GetE() IExprContext { return s.e }

func (s *NestedContext) SetE(v IExprContext) { s.e = v }

func (s *NestedContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NestedContext) LPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserLPAREN, 0)
}

func (s *NestedContext) RPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserRPAREN, 0)
}

func (s *NestedContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *NestedContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterNested(s)
	}
}

func (s *NestedContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitNested(s)
	}
}

func (s *NestedContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitNested(s)

	default:
		return t.VisitChildren(s)
	}
}

type CreateMessageContext struct {
	PrimaryContext
	leadingDot  antlr.Token
	_IDENTIFIER antlr.Token
	ids         []antlr.Token
	s30         antlr.Token
	ops         []antlr.Token
	op          antlr.Token
	entries     IFieldInitializerListContext
}

func NewCreateMessageContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *CreateMessageContext {
	var p = new(CreateMessageContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *CreateMessageContext) GetLeadingDot() antlr.Token { return s.leadingDot }

func (s *CreateMessageContext) Get_IDENTIFIER() antlr.Token { return s._IDENTIFIER }

func (s *CreateMessageContext) GetS30() antlr.Token { return s.s30 }

func (s *CreateMessageContext) GetOp() antlr.Token { return s.op }

func (s *CreateMessageContext) SetLeadingDot(v antlr.Token) { s.leadingDot = v }

func (s *CreateMessageContext) Set_IDENTIFIER(v antlr.Token) { s._IDENTIFIER = v }

func (s *CreateMessageContext) SetS30(v antlr.Token) { s.s30 = v }

func (s *CreateMessageContext) SetOp(v antlr.Token) { s.op = v }

func (s *CreateMessageContext) GetIds() []antlr.Token { return s.ids }

func (s *CreateMessageContext) GetOps() []antlr.Token { return s.ops }

func (s *CreateMessageContext) SetIds(v []antlr.Token) { s.ids = v }

func (s *CreateMessageContext) SetOps(v []antlr.Token) { s.ops = v }

func (s *CreateMessageContext) GetEntries() IFieldInitializerListContext { return s.entries }

func (s *CreateMessageContext) SetEntries(v IFieldInitializerListContext) { s.entries = v }

func (s *CreateMessageContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *CreateMessageContext) RBRACE() antlr.TerminalNode {
	return s.GetToken(CommandsParserRBRACE, 0)
}

func (s *CreateMessageContext) AllIDENTIFIER() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserIDENTIFIER)
}

func (s *CreateMessageContext) IDENTIFIER(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, i)
}

func (s *CreateMessageContext) LBRACE() antlr.TerminalNode {
	return s.GetToken(CommandsParserLBRACE, 0)
}

func (s *CreateMessageContext) COMMA() antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, 0)
}

func (s *CreateMessageContext) AllDOT() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserDOT)
}

func (s *CreateMessageContext) DOT(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, i)
}

func (s *CreateMessageContext) FieldInitializerList() IFieldInitializerListContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IFieldInitializerListContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IFieldInitializerListContext)
}

func (s *CreateMessageContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterCreateMessage(s)
	}
}

func (s *CreateMessageContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitCreateMessage(s)
	}
}

func (s *CreateMessageContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitCreateMessage(s)

	default:
		return t.VisitChildren(s)
	}
}

type GlobalCallContext struct {
	PrimaryContext
	leadingDot antlr.Token
	id         antlr.Token
	op         antlr.Token
	args       IExprListContext
}

func NewGlobalCallContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *GlobalCallContext {
	var p = new(GlobalCallContext)

	InitEmptyPrimaryContext(&p.PrimaryContext)
	p.parser = parser
	p.CopyAll(ctx.(*PrimaryContext))

	return p
}

func (s *GlobalCallContext) GetLeadingDot() antlr.Token { return s.leadingDot }

func (s *GlobalCallContext) GetId() antlr.Token { return s.id }

func (s *GlobalCallContext) GetOp() antlr.Token { return s.op }

func (s *GlobalCallContext) SetLeadingDot(v antlr.Token) { s.leadingDot = v }

func (s *GlobalCallContext) SetId(v antlr.Token) { s.id = v }

func (s *GlobalCallContext) SetOp(v antlr.Token) { s.op = v }

func (s *GlobalCallContext) GetArgs() IExprListContext { return s.args }

func (s *GlobalCallContext) SetArgs(v IExprListContext) { s.args = v }

func (s *GlobalCallContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *GlobalCallContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, 0)
}

func (s *GlobalCallContext) RPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserRPAREN, 0)
}

func (s *GlobalCallContext) LPAREN() antlr.TerminalNode {
	return s.GetToken(CommandsParserLPAREN, 0)
}

func (s *GlobalCallContext) DOT() antlr.TerminalNode {
	return s.GetToken(CommandsParserDOT, 0)
}

func (s *GlobalCallContext) ExprList() IExprListContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprListContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprListContext)
}

func (s *GlobalCallContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterGlobalCall(s)
	}
}

func (s *GlobalCallContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitGlobalCall(s)
	}
}

func (s *GlobalCallContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitGlobalCall(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Primary() (localctx IPrimaryContext) {
	localctx = NewPrimaryContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 56, CommandsParserRULE_primary)
	var _la int

	p.SetState(364)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 45, p.GetParserRuleContext()) {
	case 1:
		localctx = NewIdentContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		p.SetState(312)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserDOT {
			{
				p.SetState(311)

				var _m = p.Match(CommandsParserDOT)

				localctx.(*IdentContext).leadingDot = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(314)

			var _m = p.Match(CommandsParserIDENTIFIER)

			localctx.(*IdentContext).id = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 2:
		localctx = NewGlobalCallContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		p.SetState(316)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserDOT {
			{
				p.SetState(315)

				var _m = p.Match(CommandsParserDOT)

				localctx.(*GlobalCallContext).leadingDot = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(318)

			var _m = p.Match(CommandsParserIDENTIFIER)

			localctx.(*GlobalCallContext).id = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

		{
			p.SetState(319)

			var _m = p.Match(CommandsParserLPAREN)

			localctx.(*GlobalCallContext).op = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(321)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2224326333956096) != 0 {
			{
				p.SetState(320)

				var _x = p.ExprList()

				localctx.(*GlobalCallContext).args = _x
			}

		}
		{
			p.SetState(323)
			p.Match(CommandsParserRPAREN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 3:
		localctx = NewNestedContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(324)
			p.Match(CommandsParserLPAREN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(325)

			var _x = p.Expr()

			localctx.(*NestedContext).e = _x
		}
		{
			p.SetState(326)
			p.Match(CommandsParserRPAREN)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 4:
		localctx = NewCreateListContext(p, localctx)
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(328)

			var _m = p.Match(CommandsParserLBRACKET)

			localctx.(*CreateListContext).op = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(330)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2224343513825280) != 0 {
			{
				p.SetState(329)

				var _x = p.ListInit()

				localctx.(*CreateListContext).elems = _x
			}

		}
		p.SetState(333)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserCOMMA {
			{
				p.SetState(332)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(335)
			p.Match(CommandsParserRPRACKET)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 5:
		localctx = NewCreateStructContext(p, localctx)
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(336)

			var _m = p.Match(CommandsParserLBRACE)

			localctx.(*CreateStructContext).op = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(338)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&2224343513825280) != 0 {
			{
				p.SetState(337)

				var _x = p.MapInitializerList()

				localctx.(*CreateStructContext).entries = _x
			}

		}
		p.SetState(341)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserCOMMA {
			{
				p.SetState(340)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(343)
			p.Match(CommandsParserRBRACE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 6:
		localctx = NewCreateMessageContext(p, localctx)
		p.EnterOuterAlt(localctx, 6)
		p.SetState(345)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserDOT {
			{
				p.SetState(344)

				var _m = p.Match(CommandsParserDOT)

				localctx.(*CreateMessageContext).leadingDot = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(347)

			var _m = p.Match(CommandsParserIDENTIFIER)

			localctx.(*CreateMessageContext)._IDENTIFIER = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		localctx.(*CreateMessageContext).ids = append(localctx.(*CreateMessageContext).ids, localctx.(*CreateMessageContext)._IDENTIFIER)
		p.SetState(352)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == CommandsParserDOT {
			{
				p.SetState(348)

				var _m = p.Match(CommandsParserDOT)

				localctx.(*CreateMessageContext).s30 = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*CreateMessageContext).ops = append(localctx.(*CreateMessageContext).ops, localctx.(*CreateMessageContext).s30)
			{
				p.SetState(349)

				var _m = p.Match(CommandsParserIDENTIFIER)

				localctx.(*CreateMessageContext)._IDENTIFIER = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*CreateMessageContext).ids = append(localctx.(*CreateMessageContext).ids, localctx.(*CreateMessageContext)._IDENTIFIER)

			p.SetState(354)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}
		{
			p.SetState(355)

			var _m = p.Match(CommandsParserLBRACE)

			localctx.(*CreateMessageContext).op = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(357)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&3377716900397056) != 0 {
			{
				p.SetState(356)

				var _x = p.FieldInitializerList()

				localctx.(*CreateMessageContext).entries = _x
			}

		}
		p.SetState(360)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserCOMMA {
			{
				p.SetState(359)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(362)
			p.Match(CommandsParserRBRACE)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 7:
		localctx = NewConstantLiteralContext(p, localctx)
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(363)
			p.Literal()
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IExprListContext is an interface to support dynamic dispatch.
type IExprListContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Get_expr returns the _expr rule contexts.
	Get_expr() IExprContext

	// Set_expr sets the _expr rule contexts.
	Set_expr(IExprContext)

	// GetE returns the e rule context list.
	GetE() []IExprContext

	// SetE sets the e rule context list.
	SetE([]IExprContext)

	// Getter signatures
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsExprListContext differentiates from other interfaces.
	IsExprListContext()
}

type ExprListContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	_expr  IExprContext
	e      []IExprContext
}

func NewEmptyExprListContext() *ExprListContext {
	var p = new(ExprListContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_exprList
	return p
}

func InitEmptyExprListContext(p *ExprListContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_exprList
}

func (*ExprListContext) IsExprListContext() {}

func NewExprListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ExprListContext {
	var p = new(ExprListContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_exprList

	return p
}

func (s *ExprListContext) GetParser() antlr.Parser { return s.parser }

func (s *ExprListContext) Get_expr() IExprContext { return s._expr }

func (s *ExprListContext) Set_expr(v IExprContext) { s._expr = v }

func (s *ExprListContext) GetE() []IExprContext { return s.e }

func (s *ExprListContext) SetE(v []IExprContext) { s.e = v }

func (s *ExprListContext) AllExpr() []IExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExprContext); ok {
			len++
		}
	}

	tst := make([]IExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExprContext); ok {
			tst[i] = t.(IExprContext)
			i++
		}
	}

	return tst
}

func (s *ExprListContext) Expr(i int) IExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *ExprListContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOMMA)
}

func (s *ExprListContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, i)
}

func (s *ExprListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ExprListContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ExprListContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterExprList(s)
	}
}

func (s *ExprListContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitExprList(s)
	}
}

func (s *ExprListContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitExprList(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) ExprList() (localctx IExprListContext) {
	localctx = NewExprListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 58, CommandsParserRULE_exprList)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(366)

		var _x = p.Expr()

		localctx.(*ExprListContext)._expr = _x
	}
	localctx.(*ExprListContext).e = append(localctx.(*ExprListContext).e, localctx.(*ExprListContext)._expr)
	p.SetState(371)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	for _la == CommandsParserCOMMA {
		{
			p.SetState(367)
			p.Match(CommandsParserCOMMA)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(368)

			var _x = p.Expr()

			localctx.(*ExprListContext)._expr = _x
		}
		localctx.(*ExprListContext).e = append(localctx.(*ExprListContext).e, localctx.(*ExprListContext)._expr)

		p.SetState(373)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IListInitContext is an interface to support dynamic dispatch.
type IListInitContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Get_optExpr returns the _optExpr rule contexts.
	Get_optExpr() IOptExprContext

	// Set_optExpr sets the _optExpr rule contexts.
	Set_optExpr(IOptExprContext)

	// GetElems returns the elems rule context list.
	GetElems() []IOptExprContext

	// SetElems sets the elems rule context list.
	SetElems([]IOptExprContext)

	// Getter signatures
	AllOptExpr() []IOptExprContext
	OptExpr(i int) IOptExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsListInitContext differentiates from other interfaces.
	IsListInitContext()
}

type ListInitContext struct {
	antlr.BaseParserRuleContext
	parser   antlr.Parser
	_optExpr IOptExprContext
	elems    []IOptExprContext
}

func NewEmptyListInitContext() *ListInitContext {
	var p = new(ListInitContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_listInit
	return p
}

func InitEmptyListInitContext(p *ListInitContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_listInit
}

func (*ListInitContext) IsListInitContext() {}

func NewListInitContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ListInitContext {
	var p = new(ListInitContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_listInit

	return p
}

func (s *ListInitContext) GetParser() antlr.Parser { return s.parser }

func (s *ListInitContext) Get_optExpr() IOptExprContext { return s._optExpr }

func (s *ListInitContext) Set_optExpr(v IOptExprContext) { s._optExpr = v }

func (s *ListInitContext) GetElems() []IOptExprContext { return s.elems }

func (s *ListInitContext) SetElems(v []IOptExprContext) { s.elems = v }

func (s *ListInitContext) AllOptExpr() []IOptExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IOptExprContext); ok {
			len++
		}
	}

	tst := make([]IOptExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IOptExprContext); ok {
			tst[i] = t.(IOptExprContext)
			i++
		}
	}

	return tst
}

func (s *ListInitContext) OptExpr(i int) IOptExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOptExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOptExprContext)
}

func (s *ListInitContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOMMA)
}

func (s *ListInitContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, i)
}

func (s *ListInitContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ListInitContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ListInitContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterListInit(s)
	}
}

func (s *ListInitContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitListInit(s)
	}
}

func (s *ListInitContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitListInit(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) ListInit() (localctx IListInitContext) {
	localctx = NewListInitContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 60, CommandsParserRULE_listInit)
	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(374)

		var _x = p.OptExpr()

		localctx.(*ListInitContext)._optExpr = _x
	}
	localctx.(*ListInitContext).elems = append(localctx.(*ListInitContext).elems, localctx.(*ListInitContext)._optExpr)
	p.SetState(379)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 47, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			{
				p.SetState(375)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(376)

				var _x = p.OptExpr()

				localctx.(*ListInitContext)._optExpr = _x
			}
			localctx.(*ListInitContext).elems = append(localctx.(*ListInitContext).elems, localctx.(*ListInitContext)._optExpr)

		}
		p.SetState(381)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 47, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IFieldInitializerListContext is an interface to support dynamic dispatch.
type IFieldInitializerListContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetS35 returns the s35 token.
	GetS35() antlr.Token

	// SetS35 sets the s35 token.
	SetS35(antlr.Token)

	// GetCols returns the cols token list.
	GetCols() []antlr.Token

	// SetCols sets the cols token list.
	SetCols([]antlr.Token)

	// Get_optField returns the _optField rule contexts.
	Get_optField() IOptFieldContext

	// Get_expr returns the _expr rule contexts.
	Get_expr() IExprContext

	// Set_optField sets the _optField rule contexts.
	Set_optField(IOptFieldContext)

	// Set_expr sets the _expr rule contexts.
	Set_expr(IExprContext)

	// GetFields returns the fields rule context list.
	GetFields() []IOptFieldContext

	// GetValues returns the values rule context list.
	GetValues() []IExprContext

	// SetFields sets the fields rule context list.
	SetFields([]IOptFieldContext)

	// SetValues sets the values rule context list.
	SetValues([]IExprContext)

	// Getter signatures
	AllOptField() []IOptFieldContext
	OptField(i int) IOptFieldContext
	AllCOLON() []antlr.TerminalNode
	COLON(i int) antlr.TerminalNode
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsFieldInitializerListContext differentiates from other interfaces.
	IsFieldInitializerListContext()
}

type FieldInitializerListContext struct {
	antlr.BaseParserRuleContext
	parser    antlr.Parser
	_optField IOptFieldContext
	fields    []IOptFieldContext
	s35       antlr.Token
	cols      []antlr.Token
	_expr     IExprContext
	values    []IExprContext
}

func NewEmptyFieldInitializerListContext() *FieldInitializerListContext {
	var p = new(FieldInitializerListContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_fieldInitializerList
	return p
}

func InitEmptyFieldInitializerListContext(p *FieldInitializerListContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_fieldInitializerList
}

func (*FieldInitializerListContext) IsFieldInitializerListContext() {}

func NewFieldInitializerListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *FieldInitializerListContext {
	var p = new(FieldInitializerListContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_fieldInitializerList

	return p
}

func (s *FieldInitializerListContext) GetParser() antlr.Parser { return s.parser }

func (s *FieldInitializerListContext) GetS35() antlr.Token { return s.s35 }

func (s *FieldInitializerListContext) SetS35(v antlr.Token) { s.s35 = v }

func (s *FieldInitializerListContext) GetCols() []antlr.Token { return s.cols }

func (s *FieldInitializerListContext) SetCols(v []antlr.Token) { s.cols = v }

func (s *FieldInitializerListContext) Get_optField() IOptFieldContext { return s._optField }

func (s *FieldInitializerListContext) Get_expr() IExprContext { return s._expr }

func (s *FieldInitializerListContext) Set_optField(v IOptFieldContext) { s._optField = v }

func (s *FieldInitializerListContext) Set_expr(v IExprContext) { s._expr = v }

func (s *FieldInitializerListContext) GetFields() []IOptFieldContext { return s.fields }

func (s *FieldInitializerListContext) GetValues() []IExprContext { return s.values }

func (s *FieldInitializerListContext) SetFields(v []IOptFieldContext) { s.fields = v }

func (s *FieldInitializerListContext) SetValues(v []IExprContext) { s.values = v }

func (s *FieldInitializerListContext) AllOptField() []IOptFieldContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IOptFieldContext); ok {
			len++
		}
	}

	tst := make([]IOptFieldContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IOptFieldContext); ok {
			tst[i] = t.(IOptFieldContext)
			i++
		}
	}

	return tst
}

func (s *FieldInitializerListContext) OptField(i int) IOptFieldContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOptFieldContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOptFieldContext)
}

func (s *FieldInitializerListContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOLON)
}

func (s *FieldInitializerListContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOLON, i)
}

func (s *FieldInitializerListContext) AllExpr() []IExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExprContext); ok {
			len++
		}
	}

	tst := make([]IExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExprContext); ok {
			tst[i] = t.(IExprContext)
			i++
		}
	}

	return tst
}

func (s *FieldInitializerListContext) Expr(i int) IExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *FieldInitializerListContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOMMA)
}

func (s *FieldInitializerListContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, i)
}

func (s *FieldInitializerListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *FieldInitializerListContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *FieldInitializerListContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterFieldInitializerList(s)
	}
}

func (s *FieldInitializerListContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitFieldInitializerList(s)
	}
}

func (s *FieldInitializerListContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitFieldInitializerList(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) FieldInitializerList() (localctx IFieldInitializerListContext) {
	localctx = NewFieldInitializerListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 62, CommandsParserRULE_fieldInitializerList)
	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(382)

		var _x = p.OptField()

		localctx.(*FieldInitializerListContext)._optField = _x
	}
	localctx.(*FieldInitializerListContext).fields = append(localctx.(*FieldInitializerListContext).fields, localctx.(*FieldInitializerListContext)._optField)
	{
		p.SetState(383)

		var _m = p.Match(CommandsParserCOLON)

		localctx.(*FieldInitializerListContext).s35 = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	localctx.(*FieldInitializerListContext).cols = append(localctx.(*FieldInitializerListContext).cols, localctx.(*FieldInitializerListContext).s35)
	{
		p.SetState(384)

		var _x = p.Expr()

		localctx.(*FieldInitializerListContext)._expr = _x
	}
	localctx.(*FieldInitializerListContext).values = append(localctx.(*FieldInitializerListContext).values, localctx.(*FieldInitializerListContext)._expr)
	p.SetState(392)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 48, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			{
				p.SetState(385)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(386)

				var _x = p.OptField()

				localctx.(*FieldInitializerListContext)._optField = _x
			}
			localctx.(*FieldInitializerListContext).fields = append(localctx.(*FieldInitializerListContext).fields, localctx.(*FieldInitializerListContext)._optField)
			{
				p.SetState(387)

				var _m = p.Match(CommandsParserCOLON)

				localctx.(*FieldInitializerListContext).s35 = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*FieldInitializerListContext).cols = append(localctx.(*FieldInitializerListContext).cols, localctx.(*FieldInitializerListContext).s35)
			{
				p.SetState(388)

				var _x = p.Expr()

				localctx.(*FieldInitializerListContext)._expr = _x
			}
			localctx.(*FieldInitializerListContext).values = append(localctx.(*FieldInitializerListContext).values, localctx.(*FieldInitializerListContext)._expr)

		}
		p.SetState(394)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 48, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IOptFieldContext is an interface to support dynamic dispatch.
type IOptFieldContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetOpt returns the opt token.
	GetOpt() antlr.Token

	// SetOpt sets the opt token.
	SetOpt(antlr.Token)

	// Getter signatures
	EscapeIdent() IEscapeIdentContext
	QUESTIONMARK() antlr.TerminalNode

	// IsOptFieldContext differentiates from other interfaces.
	IsOptFieldContext()
}

type OptFieldContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	opt    antlr.Token
}

func NewEmptyOptFieldContext() *OptFieldContext {
	var p = new(OptFieldContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_optField
	return p
}

func InitEmptyOptFieldContext(p *OptFieldContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_optField
}

func (*OptFieldContext) IsOptFieldContext() {}

func NewOptFieldContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OptFieldContext {
	var p = new(OptFieldContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_optField

	return p
}

func (s *OptFieldContext) GetParser() antlr.Parser { return s.parser }

func (s *OptFieldContext) GetOpt() antlr.Token { return s.opt }

func (s *OptFieldContext) SetOpt(v antlr.Token) { s.opt = v }

func (s *OptFieldContext) EscapeIdent() IEscapeIdentContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEscapeIdentContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEscapeIdentContext)
}

func (s *OptFieldContext) QUESTIONMARK() antlr.TerminalNode {
	return s.GetToken(CommandsParserQUESTIONMARK, 0)
}

func (s *OptFieldContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *OptFieldContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *OptFieldContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterOptField(s)
	}
}

func (s *OptFieldContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitOptField(s)
	}
}

func (s *OptFieldContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitOptField(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) OptField() (localctx IOptFieldContext) {
	localctx = NewOptFieldContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 64, CommandsParserRULE_optField)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(396)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserQUESTIONMARK {
		{
			p.SetState(395)

			var _m = p.Match(CommandsParserQUESTIONMARK)

			localctx.(*OptFieldContext).opt = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(398)
		p.EscapeIdent()
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IMapInitializerListContext is an interface to support dynamic dispatch.
type IMapInitializerListContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetS35 returns the s35 token.
	GetS35() antlr.Token

	// SetS35 sets the s35 token.
	SetS35(antlr.Token)

	// GetCols returns the cols token list.
	GetCols() []antlr.Token

	// SetCols sets the cols token list.
	SetCols([]antlr.Token)

	// Get_optExpr returns the _optExpr rule contexts.
	Get_optExpr() IOptExprContext

	// Get_expr returns the _expr rule contexts.
	Get_expr() IExprContext

	// Set_optExpr sets the _optExpr rule contexts.
	Set_optExpr(IOptExprContext)

	// Set_expr sets the _expr rule contexts.
	Set_expr(IExprContext)

	// GetKeys returns the keys rule context list.
	GetKeys() []IOptExprContext

	// GetValues returns the values rule context list.
	GetValues() []IExprContext

	// SetKeys sets the keys rule context list.
	SetKeys([]IOptExprContext)

	// SetValues sets the values rule context list.
	SetValues([]IExprContext)

	// Getter signatures
	AllOptExpr() []IOptExprContext
	OptExpr(i int) IOptExprContext
	AllCOLON() []antlr.TerminalNode
	COLON(i int) antlr.TerminalNode
	AllExpr() []IExprContext
	Expr(i int) IExprContext
	AllCOMMA() []antlr.TerminalNode
	COMMA(i int) antlr.TerminalNode

	// IsMapInitializerListContext differentiates from other interfaces.
	IsMapInitializerListContext()
}

type MapInitializerListContext struct {
	antlr.BaseParserRuleContext
	parser   antlr.Parser
	_optExpr IOptExprContext
	keys     []IOptExprContext
	s35      antlr.Token
	cols     []antlr.Token
	_expr    IExprContext
	values   []IExprContext
}

func NewEmptyMapInitializerListContext() *MapInitializerListContext {
	var p = new(MapInitializerListContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_mapInitializerList
	return p
}

func InitEmptyMapInitializerListContext(p *MapInitializerListContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_mapInitializerList
}

func (*MapInitializerListContext) IsMapInitializerListContext() {}

func NewMapInitializerListContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *MapInitializerListContext {
	var p = new(MapInitializerListContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_mapInitializerList

	return p
}

func (s *MapInitializerListContext) GetParser() antlr.Parser { return s.parser }

func (s *MapInitializerListContext) GetS35() antlr.Token { return s.s35 }

func (s *MapInitializerListContext) SetS35(v antlr.Token) { s.s35 = v }

func (s *MapInitializerListContext) GetCols() []antlr.Token { return s.cols }

func (s *MapInitializerListContext) SetCols(v []antlr.Token) { s.cols = v }

func (s *MapInitializerListContext) Get_optExpr() IOptExprContext { return s._optExpr }

func (s *MapInitializerListContext) Get_expr() IExprContext { return s._expr }

func (s *MapInitializerListContext) Set_optExpr(v IOptExprContext) { s._optExpr = v }

func (s *MapInitializerListContext) Set_expr(v IExprContext) { s._expr = v }

func (s *MapInitializerListContext) GetKeys() []IOptExprContext { return s.keys }

func (s *MapInitializerListContext) GetValues() []IExprContext { return s.values }

func (s *MapInitializerListContext) SetKeys(v []IOptExprContext) { s.keys = v }

func (s *MapInitializerListContext) SetValues(v []IExprContext) { s.values = v }

func (s *MapInitializerListContext) AllOptExpr() []IOptExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IOptExprContext); ok {
			len++
		}
	}

	tst := make([]IOptExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IOptExprContext); ok {
			tst[i] = t.(IOptExprContext)
			i++
		}
	}

	return tst
}

func (s *MapInitializerListContext) OptExpr(i int) IOptExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IOptExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IOptExprContext)
}

func (s *MapInitializerListContext) AllCOLON() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOLON)
}

func (s *MapInitializerListContext) COLON(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOLON, i)
}

func (s *MapInitializerListContext) AllExpr() []IExprContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IExprContext); ok {
			len++
		}
	}

	tst := make([]IExprContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IExprContext); ok {
			tst[i] = t.(IExprContext)
			i++
		}
	}

	return tst
}

func (s *MapInitializerListContext) Expr(i int) IExprContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *MapInitializerListContext) AllCOMMA() []antlr.TerminalNode {
	return s.GetTokens(CommandsParserCOMMA)
}

func (s *MapInitializerListContext) COMMA(i int) antlr.TerminalNode {
	return s.GetToken(CommandsParserCOMMA, i)
}

func (s *MapInitializerListContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *MapInitializerListContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *MapInitializerListContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterMapInitializerList(s)
	}
}

func (s *MapInitializerListContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitMapInitializerList(s)
	}
}

func (s *MapInitializerListContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitMapInitializerList(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) MapInitializerList() (localctx IMapInitializerListContext) {
	localctx = NewMapInitializerListContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 66, CommandsParserRULE_mapInitializerList)
	var _alt int

	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(400)

		var _x = p.OptExpr()

		localctx.(*MapInitializerListContext)._optExpr = _x
	}
	localctx.(*MapInitializerListContext).keys = append(localctx.(*MapInitializerListContext).keys, localctx.(*MapInitializerListContext)._optExpr)
	{
		p.SetState(401)

		var _m = p.Match(CommandsParserCOLON)

		localctx.(*MapInitializerListContext).s35 = _m
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	localctx.(*MapInitializerListContext).cols = append(localctx.(*MapInitializerListContext).cols, localctx.(*MapInitializerListContext).s35)
	{
		p.SetState(402)

		var _x = p.Expr()

		localctx.(*MapInitializerListContext)._expr = _x
	}
	localctx.(*MapInitializerListContext).values = append(localctx.(*MapInitializerListContext).values, localctx.(*MapInitializerListContext)._expr)
	p.SetState(410)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 50, p.GetParserRuleContext())
	if p.HasError() {
		goto errorExit
	}
	for _alt != 2 && _alt != antlr.ATNInvalidAltNumber {
		if _alt == 1 {
			{
				p.SetState(403)
				p.Match(CommandsParserCOMMA)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(404)

				var _x = p.OptExpr()

				localctx.(*MapInitializerListContext)._optExpr = _x
			}
			localctx.(*MapInitializerListContext).keys = append(localctx.(*MapInitializerListContext).keys, localctx.(*MapInitializerListContext)._optExpr)
			{
				p.SetState(405)

				var _m = p.Match(CommandsParserCOLON)

				localctx.(*MapInitializerListContext).s35 = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			localctx.(*MapInitializerListContext).cols = append(localctx.(*MapInitializerListContext).cols, localctx.(*MapInitializerListContext).s35)
			{
				p.SetState(406)

				var _x = p.Expr()

				localctx.(*MapInitializerListContext)._expr = _x
			}
			localctx.(*MapInitializerListContext).values = append(localctx.(*MapInitializerListContext).values, localctx.(*MapInitializerListContext)._expr)

		}
		p.SetState(412)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_alt = p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 50, p.GetParserRuleContext())
		if p.HasError() {
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IEscapeIdentContext is an interface to support dynamic dispatch.
type IEscapeIdentContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsEscapeIdentContext differentiates from other interfaces.
	IsEscapeIdentContext()
}

type EscapeIdentContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEscapeIdentContext() *EscapeIdentContext {
	var p = new(EscapeIdentContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_escapeIdent
	return p
}

func InitEmptyEscapeIdentContext(p *EscapeIdentContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_escapeIdent
}

func (*EscapeIdentContext) IsEscapeIdentContext() {}

func NewEscapeIdentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EscapeIdentContext {
	var p = new(EscapeIdentContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_escapeIdent

	return p
}

func (s *EscapeIdentContext) GetParser() antlr.Parser { return s.parser }

func (s *EscapeIdentContext) CopyAll(ctx *EscapeIdentContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *EscapeIdentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EscapeIdentContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type EscapedIdentifierContext struct {
	EscapeIdentContext
	id antlr.Token
}

func NewEscapedIdentifierContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *EscapedIdentifierContext {
	var p = new(EscapedIdentifierContext)

	InitEmptyEscapeIdentContext(&p.EscapeIdentContext)
	p.parser = parser
	p.CopyAll(ctx.(*EscapeIdentContext))

	return p
}

func (s *EscapedIdentifierContext) GetId() antlr.Token { return s.id }

func (s *EscapedIdentifierContext) SetId(v antlr.Token) { s.id = v }

func (s *EscapedIdentifierContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EscapedIdentifierContext) ESC_IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserESC_IDENTIFIER, 0)
}

func (s *EscapedIdentifierContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterEscapedIdentifier(s)
	}
}

func (s *EscapedIdentifierContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitEscapedIdentifier(s)
	}
}

func (s *EscapedIdentifierContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitEscapedIdentifier(s)

	default:
		return t.VisitChildren(s)
	}
}

type SimpleIdentifierContext struct {
	EscapeIdentContext
	id antlr.Token
}

func NewSimpleIdentifierContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *SimpleIdentifierContext {
	var p = new(SimpleIdentifierContext)

	InitEmptyEscapeIdentContext(&p.EscapeIdentContext)
	p.parser = parser
	p.CopyAll(ctx.(*EscapeIdentContext))

	return p
}

func (s *SimpleIdentifierContext) GetId() antlr.Token { return s.id }

func (s *SimpleIdentifierContext) SetId(v antlr.Token) { s.id = v }

func (s *SimpleIdentifierContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *SimpleIdentifierContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(CommandsParserIDENTIFIER, 0)
}

func (s *SimpleIdentifierContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterSimpleIdentifier(s)
	}
}

func (s *SimpleIdentifierContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitSimpleIdentifier(s)
	}
}

func (s *SimpleIdentifierContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitSimpleIdentifier(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) EscapeIdent() (localctx IEscapeIdentContext) {
	localctx = NewEscapeIdentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 68, CommandsParserRULE_escapeIdent)
	p.SetState(415)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case CommandsParserIDENTIFIER:
		localctx = NewSimpleIdentifierContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(413)

			var _m = p.Match(CommandsParserIDENTIFIER)

			localctx.(*SimpleIdentifierContext).id = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case CommandsParserESC_IDENTIFIER:
		localctx = NewEscapedIdentifierContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(414)

			var _m = p.Match(CommandsParserESC_IDENTIFIER)

			localctx.(*EscapedIdentifierContext).id = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IOptExprContext is an interface to support dynamic dispatch.
type IOptExprContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// GetOpt returns the opt token.
	GetOpt() antlr.Token

	// SetOpt sets the opt token.
	SetOpt(antlr.Token)

	// GetE returns the e rule contexts.
	GetE() IExprContext

	// SetE sets the e rule contexts.
	SetE(IExprContext)

	// Getter signatures
	Expr() IExprContext
	QUESTIONMARK() antlr.TerminalNode

	// IsOptExprContext differentiates from other interfaces.
	IsOptExprContext()
}

type OptExprContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
	opt    antlr.Token
	e      IExprContext
}

func NewEmptyOptExprContext() *OptExprContext {
	var p = new(OptExprContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_optExpr
	return p
}

func InitEmptyOptExprContext(p *OptExprContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_optExpr
}

func (*OptExprContext) IsOptExprContext() {}

func NewOptExprContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *OptExprContext {
	var p = new(OptExprContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_optExpr

	return p
}

func (s *OptExprContext) GetParser() antlr.Parser { return s.parser }

func (s *OptExprContext) GetOpt() antlr.Token { return s.opt }

func (s *OptExprContext) SetOpt(v antlr.Token) { s.opt = v }

func (s *OptExprContext) GetE() IExprContext { return s.e }

func (s *OptExprContext) SetE(v IExprContext) { s.e = v }

func (s *OptExprContext) Expr() IExprContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IExprContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IExprContext)
}

func (s *OptExprContext) QUESTIONMARK() antlr.TerminalNode {
	return s.GetToken(CommandsParserQUESTIONMARK, 0)
}

func (s *OptExprContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *OptExprContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *OptExprContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterOptExpr(s)
	}
}

func (s *OptExprContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitOptExpr(s)
	}
}

func (s *OptExprContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitOptExpr(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) OptExpr() (localctx IOptExprContext) {
	localctx = NewOptExprContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 70, CommandsParserRULE_optExpr)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(418)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == CommandsParserQUESTIONMARK {
		{
			p.SetState(417)

			var _m = p.Match(CommandsParserQUESTIONMARK)

			localctx.(*OptExprContext).opt = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	}
	{
		p.SetState(420)

		var _x = p.Expr()

		localctx.(*OptExprContext).e = _x
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ILiteralContext is an interface to support dynamic dispatch.
type ILiteralContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsLiteralContext differentiates from other interfaces.
	IsLiteralContext()
}

type LiteralContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyLiteralContext() *LiteralContext {
	var p = new(LiteralContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_literal
	return p
}

func InitEmptyLiteralContext(p *LiteralContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = CommandsParserRULE_literal
}

func (*LiteralContext) IsLiteralContext() {}

func NewLiteralContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *LiteralContext {
	var p = new(LiteralContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = CommandsParserRULE_literal

	return p
}

func (s *LiteralContext) GetParser() antlr.Parser { return s.parser }

func (s *LiteralContext) CopyAll(ctx *LiteralContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *LiteralContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *LiteralContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type BytesContext struct {
	LiteralContext
	tok antlr.Token
}

func NewBytesContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BytesContext {
	var p = new(BytesContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *BytesContext) GetTok() antlr.Token { return s.tok }

func (s *BytesContext) SetTok(v antlr.Token) { s.tok = v }

func (s *BytesContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BytesContext) BYTES() antlr.TerminalNode {
	return s.GetToken(CommandsParserBYTES, 0)
}

func (s *BytesContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterBytes(s)
	}
}

func (s *BytesContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitBytes(s)
	}
}

func (s *BytesContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitBytes(s)

	default:
		return t.VisitChildren(s)
	}
}

type UintContext struct {
	LiteralContext
	tok antlr.Token
}

func NewUintContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *UintContext {
	var p = new(UintContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *UintContext) GetTok() antlr.Token { return s.tok }

func (s *UintContext) SetTok(v antlr.Token) { s.tok = v }

func (s *UintContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UintContext) NUM_UINT() antlr.TerminalNode {
	return s.GetToken(CommandsParserNUM_UINT, 0)
}

func (s *UintContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterUint(s)
	}
}

func (s *UintContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitUint(s)
	}
}

func (s *UintContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitUint(s)

	default:
		return t.VisitChildren(s)
	}
}

type NullContext struct {
	LiteralContext
	tok antlr.Token
}

func NewNullContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *NullContext {
	var p = new(NullContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *NullContext) GetTok() antlr.Token { return s.tok }

func (s *NullContext) SetTok(v antlr.Token) { s.tok = v }

func (s *NullContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *NullContext) NUL() antlr.TerminalNode {
	return s.GetToken(CommandsParserNUL, 0)
}

func (s *NullContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterNull(s)
	}
}

func (s *NullContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitNull(s)
	}
}

func (s *NullContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitNull(s)

	default:
		return t.VisitChildren(s)
	}
}

type BoolFalseContext struct {
	LiteralContext
	tok antlr.Token
}

func NewBoolFalseContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BoolFalseContext {
	var p = new(BoolFalseContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *BoolFalseContext) GetTok() antlr.Token { return s.tok }

func (s *BoolFalseContext) SetTok(v antlr.Token) { s.tok = v }

func (s *BoolFalseContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BoolFalseContext) CEL_FALSE() antlr.TerminalNode {
	return s.GetToken(CommandsParserCEL_FALSE, 0)
}

func (s *BoolFalseContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterBoolFalse(s)
	}
}

func (s *BoolFalseContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitBoolFalse(s)
	}
}

func (s *BoolFalseContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitBoolFalse(s)

	default:
		return t.VisitChildren(s)
	}
}

type StringContext struct {
	LiteralContext
	tok antlr.Token
}

func NewStringContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *StringContext {
	var p = new(StringContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *StringContext) GetTok() antlr.Token { return s.tok }

func (s *StringContext) SetTok(v antlr.Token) { s.tok = v }

func (s *StringContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StringContext) STRING() antlr.TerminalNode {
	return s.GetToken(CommandsParserSTRING, 0)
}

func (s *StringContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterString(s)
	}
}

func (s *StringContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitString(s)
	}
}

func (s *StringContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitString(s)

	default:
		return t.VisitChildren(s)
	}
}

type DoubleContext struct {
	LiteralContext
	sign antlr.Token
	tok  antlr.Token
}

func NewDoubleContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *DoubleContext {
	var p = new(DoubleContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *DoubleContext) GetSign() antlr.Token { return s.sign }

func (s *DoubleContext) GetTok() antlr.Token { return s.tok }

func (s *DoubleContext) SetSign(v antlr.Token) { s.sign = v }

func (s *DoubleContext) SetTok(v antlr.Token) { s.tok = v }

func (s *DoubleContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *DoubleContext) NUM_FLOAT() antlr.TerminalNode {
	return s.GetToken(CommandsParserNUM_FLOAT, 0)
}

func (s *DoubleContext) MINUS() antlr.TerminalNode {
	return s.GetToken(CommandsParserMINUS, 0)
}

func (s *DoubleContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterDouble(s)
	}
}

func (s *DoubleContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitDouble(s)
	}
}

func (s *DoubleContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitDouble(s)

	default:
		return t.VisitChildren(s)
	}
}

type BoolTrueContext struct {
	LiteralContext
	tok antlr.Token
}

func NewBoolTrueContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *BoolTrueContext {
	var p = new(BoolTrueContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *BoolTrueContext) GetTok() antlr.Token { return s.tok }

func (s *BoolTrueContext) SetTok(v antlr.Token) { s.tok = v }

func (s *BoolTrueContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *BoolTrueContext) CEL_TRUE() antlr.TerminalNode {
	return s.GetToken(CommandsParserCEL_TRUE, 0)
}

func (s *BoolTrueContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterBoolTrue(s)
	}
}

func (s *BoolTrueContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitBoolTrue(s)
	}
}

func (s *BoolTrueContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitBoolTrue(s)

	default:
		return t.VisitChildren(s)
	}
}

type IntContext struct {
	LiteralContext
	sign antlr.Token
	tok  antlr.Token
}

func NewIntContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *IntContext {
	var p = new(IntContext)

	InitEmptyLiteralContext(&p.LiteralContext)
	p.parser = parser
	p.CopyAll(ctx.(*LiteralContext))

	return p
}

func (s *IntContext) GetSign() antlr.Token { return s.sign }

func (s *IntContext) GetTok() antlr.Token { return s.tok }

func (s *IntContext) SetSign(v antlr.Token) { s.sign = v }

func (s *IntContext) SetTok(v antlr.Token) { s.tok = v }

func (s *IntContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *IntContext) NUM_INT() antlr.TerminalNode {
	return s.GetToken(CommandsParserNUM_INT, 0)
}

func (s *IntContext) MINUS() antlr.TerminalNode {
	return s.GetToken(CommandsParserMINUS, 0)
}

func (s *IntContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.EnterInt(s)
	}
}

func (s *IntContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(CommandsListener); ok {
		listenerT.ExitInt(s)
	}
}

func (s *IntContext) Accept(visitor antlr.ParseTreeVisitor) any {
	switch t := visitor.(type) {
	case CommandsVisitor:
		return t.VisitInt(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *CommandsParser) Literal() (localctx ILiteralContext) {
	localctx = NewLiteralContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 72, CommandsParserRULE_literal)
	var _la int

	p.SetState(436)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetInterpreter().AdaptivePredict(p.BaseParser, p.GetTokenStream(), 55, p.GetParserRuleContext()) {
	case 1:
		localctx = NewIntContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		p.SetState(423)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserMINUS {
			{
				p.SetState(422)

				var _m = p.Match(CommandsParserMINUS)

				localctx.(*IntContext).sign = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(425)

			var _m = p.Match(CommandsParserNUM_INT)

			localctx.(*IntContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 2:
		localctx = NewUintContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(426)

			var _m = p.Match(CommandsParserNUM_UINT)

			localctx.(*UintContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 3:
		localctx = NewDoubleContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		p.SetState(428)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if _la == CommandsParserMINUS {
			{
				p.SetState(427)

				var _m = p.Match(CommandsParserMINUS)

				localctx.(*DoubleContext).sign = _m
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		}
		{
			p.SetState(430)

			var _m = p.Match(CommandsParserNUM_FLOAT)

			localctx.(*DoubleContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 4:
		localctx = NewStringContext(p, localctx)
		p.EnterOuterAlt(localctx, 4)
		{
			p.SetState(431)

			var _m = p.Match(CommandsParserSTRING)

			localctx.(*StringContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 5:
		localctx = NewBytesContext(p, localctx)
		p.EnterOuterAlt(localctx, 5)
		{
			p.SetState(432)

			var _m = p.Match(CommandsParserBYTES)

			localctx.(*BytesContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 6:
		localctx = NewBoolTrueContext(p, localctx)
		p.EnterOuterAlt(localctx, 6)
		{
			p.SetState(433)

			var _m = p.Match(CommandsParserCEL_TRUE)

			localctx.(*BoolTrueContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 7:
		localctx = NewBoolFalseContext(p, localctx)
		p.EnterOuterAlt(localctx, 7)
		{
			p.SetState(434)

			var _m = p.Match(CommandsParserCEL_FALSE)

			localctx.(*BoolFalseContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case 8:
		localctx = NewNullContext(p, localctx)
		p.EnterOuterAlt(localctx, 8)
		{
			p.SetState(435)

			var _m = p.Match(CommandsParserNUL)

			localctx.(*NullContext).tok = _m
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	case antlr.ATNInvalidAltNumber:
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

func (p *CommandsParser) Sempred(localctx antlr.RuleContext, ruleIndex, predIndex int) bool {
	switch ruleIndex {
	case 24:
		var t *RelationContext = nil
		if localctx != nil {
			t = localctx.(*RelationContext)
		}
		return p.Relation_Sempred(t, predIndex)

	case 25:
		var t *CalcContext = nil
		if localctx != nil {
			t = localctx.(*CalcContext)
		}
		return p.Calc_Sempred(t, predIndex)

	case 27:
		var t *MemberContext = nil
		if localctx != nil {
			t = localctx.(*MemberContext)
		}
		return p.Member_Sempred(t, predIndex)

	default:
		panic("No predicate with index: " + fmt.Sprint(ruleIndex))
	}
}

func (p *CommandsParser) Relation_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	switch predIndex {
	case 0:
		return p.Precpred(p.GetParserRuleContext(), 1)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}

func (p *CommandsParser) Calc_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	switch predIndex {
	case 1:
		return p.Precpred(p.GetParserRuleContext(), 2)

	case 2:
		return p.Precpred(p.GetParserRuleContext(), 1)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}

func (p *CommandsParser) Member_Sempred(localctx antlr.RuleContext, predIndex int) bool {
	switch predIndex {
	case 3:
		return p.Precpred(p.GetParserRuleContext(), 3)

	case 4:
		return p.Precpred(p.GetParserRuleContext(), 2)

	case 5:
		return p.Precpred(p.GetParserRuleContext(), 1)

	default:
		panic("No predicate with index: " + fmt.Sprint(predIndex))
	}
}
