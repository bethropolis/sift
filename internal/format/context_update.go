package format

import (
	"html"
	"strings"
)

// RenderContextUpdate renders the XML envelope used for raw delta patches. It
// escapes the ref attributes and splits any "]]>" in the patch body across
// CDATA sections, so a diff that itself contains XML or CDATA stays well
// formed.
func RenderContextUpdate(from, to, patch string) string {
	var b strings.Builder
	b.WriteString(`<context_update type="delta_patch" from_commit="`)
	b.WriteString(html.EscapeString(from))
	b.WriteString(`" to_commit="`)
	b.WriteString(html.EscapeString(to))
	b.WriteString("\">\n<![CDATA[\n")
	b.WriteString(escapeCDATA(patch))
	b.WriteString("\n]]>\n</context_update>\n")
	return b.String()
}

// escapeCDATA splits a CDATA terminator so it cannot close the surrounding
// section while still decoding back to the original text.
func escapeCDATA(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}
