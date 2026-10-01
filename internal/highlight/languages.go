package highlight

import (
	"strings"
	"sync"
)

type languageWordSets struct {
	keywords map[string]bool
	builtins map[string]bool
	types    map[string]bool
}

var languageWordSetCache sync.Map

func wordSetsForLanguage(language string) *languageWordSets {
	if cached, ok := languageWordSetCache.Load(language); ok {
		return cached.(*languageWordSets)
	}

	sets := &languageWordSets{
		keywords: keywordSetForLanguage(language),
		builtins: builtinSetForLanguage(language),
		types:    builtinTypeSet(language),
	}
	cached, _ := languageWordSetCache.LoadOrStore(language, sets)
	return cached.(*languageWordSets)
}

func keywordSetForLanguage(language string) map[string]bool {
	words := "if else for range switch case return func package import type struct interface class def const var let function export async await try catch throw new public private protected static void fn impl trait use mod match pub enum where namespace using map chan go defer select with yield lambda from as in is"
	if language == "dart" {
		words += " abstract as assert async await break case catch class const continue covariant default deferred do dynamic else enum export extends extension external factory false final finally for get hide if implements import in interface is late library mixin native new null of on operator part required rethrow return sealed set show static super switch sync this throw true try typedef var void while with yield"
	}
	if language == "zig" {
		words += " addrspace align allowzero and anyframe anytype asm async await bitalign break callconv catch comptime const continue defer else enum errdefer error export extern fn for if inline noalias noinline nosuspend opaque or orelse packed pub resume return linksection struct suspend switch test threadlocal try union unreachable usingnamespace var volatile while"
	}
	if language == "csharp" {
		words += " abstract as base bool break byte case catch checked char class const continue decimal default delegate do double else enum event explicit extern false finally fixed float for foreach goto if implicit in int interface internal is lock long namespace new null object operator out override params private protected public readonly ref return sbyte sealed short sizeof stackalloc static string struct switch this throw true try typeof uint ulong unchecked unsafe ushort using virtual void volatile while async await var"
	}
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
	if language == "dart" {
		words += " print debugPrint BuildContext Widget State StatelessWidget StatefulWidget Text Scaffold MaterialApp ListView Container Future Stream"
	}
	if language == "zig" {
		words += " std allocator ArrayList AutoHashMap expect panic"
	}
	return wordSet(words)
}

func builtinTypeSet(language string) map[string]bool {
	switch language {
	case "go", "rust", "python", "javascript", "typescript", "tsx", "dart", "zig", "csharp":
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
