package highlight

import "strings"

func keywordSetForLanguage(language string) map[string]bool {
	words := "if else for range switch case return func package import type struct interface class def const var let function export async await try catch throw new public private protected static void fn impl trait use mod match pub enum where namespace using map chan go defer select with yield lambda from as in is"
	return wordSet(words)
}

func builtinSetForLanguage(language string) map[string]bool {
	words := "fmt println print len cap make new append copy delete panic recover"
	if language == "python" {
		words += " self cls str int float list dict set tuple bool bytes open range enumerate isinstance super"
	}
	if language == "javascript" || language == "typescript" || language == "tsx" {
		words += " console document window Promise Array Object String Number Boolean JSON Math"
	}
	return wordSet(words)
}

func builtinTypeSet(language string) map[string]bool {
	switch language {
	case "go", "rust", "python", "javascript", "typescript", "tsx":
		return wordSet("string int bool error byte rune any i32 i64 u32 u64 usize f32 f64 str Vec Option String Number Boolean Object")
	default:
		return nil
	}
}

func wordSet(words string) map[string]bool {
	set := make(map[string]bool)
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}
