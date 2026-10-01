package highlight

import "strings"

func parseJSONLine(line string, state *scanState, allowComments bool) []Span {
	var spans []Span
	add := func(start, end int, kind TokenKind) {
		if end > start {
			spans = append(spans, Span{Start: start, End: end, Kind: kind})
		}
	}
	for i := 0; i < len(line); {
		if state.blockEnd != "" {
			end := strings.Index(line[i:], state.blockEnd)
			if end < 0 {
				add(i, len(line), TokenComment)
				break
			}
			closeAt := i + end + len(state.blockEnd)
			add(i, closeAt, TokenComment)
			i = closeAt
			state.blockEnd = ""
			continue
		}
		if line[i] == ' ' || line[i] == '\t' || line[i] == '\r' {
			i++
			continue
		}
		if allowComments && strings.HasPrefix(line[i:], "//") {
			add(i, len(line), TokenComment)
			break
		}
		if allowComments && strings.HasPrefix(line[i:], "/*") {
			if end := strings.Index(line[i+2:], "*/"); end >= 0 {
				closeAt := i + 2 + end + 2
				add(i, closeAt, TokenComment)
				i = closeAt
			} else {
				add(i, len(line), TokenComment)
				state.blockEnd = "*/"
				break
			}
			continue
		}
		if line[i] == '"' {
			start := i
			i++
			for i < len(line) {
				if line[i] == '\\' {
					i = min(len(line), i+2)
					continue
				}
				if line[i] == '"' {
					i++
					break
				}
				i++
			}
			j := i
			for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			kind := TokenString
			if j < len(line) && line[j] == ':' {
				kind = TokenProperty
			}
			add(start, i, kind)
			continue
		}
		if line[i] == '-' || line[i] >= '0' && line[i] <= '9' {
			start := i
			i++
			for i < len(line) && strings.ContainsRune("0123456789.eE+-", rune(line[i])) {
				i++
			}
			add(start, i, TokenNumber)
			continue
		}
		if line[i] == 't' && strings.HasPrefix(line[i:], "true") || line[i] == 'f' && strings.HasPrefix(line[i:], "false") {
			word := "true"
			if line[i] == 'f' {
				word = "false"
			}
			add(i, i+len(word), TokenBool)
			i += len(word)
			continue
		}
		if strings.HasPrefix(line[i:], "null") {
			add(i, i+4, TokenNull)
			i += 4
			continue
		}
		if strings.ContainsRune("{}[]:,", rune(line[i])) {
			add(i, i+1, TokenPunctuation)
		}
		i++
	}
	return spans
}

func parseYAMLLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		if trimmed == "" {
			return nil
		}
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	if strings.HasPrefix(trimmed, "- ") {
		return []Span{{Start: indent, End: indent + 2, Kind: TokenOperator}}
	}
	if colon := strings.Index(trimmed, ":"); colon >= 0 {
		return []Span{{Start: indent, End: indent + colon, Kind: TokenProperty}}
	}
	return nil
}

func parseTOMLLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if strings.HasPrefix(trimmed, "#") {
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	if strings.HasPrefix(trimmed, "[") {
		return []Span{{Start: indent, End: len(line), Kind: TokenType}}
	}
	if eq := strings.Index(trimmed, "="); eq >= 0 {
		return []Span{{Start: indent, End: indent + eq, Kind: TokenProperty}}
	}
	return nil
}

func parseHTMLLine(line string) []Span {
	var spans []Span
	for i := 0; i < len(line); {
		if line[i] != '<' {
			i++
			continue
		}
		end := strings.IndexByte(line[i:], '>')
		if end < 0 {
			spans = append(spans, Span{Start: i, End: len(line), Kind: TokenTag})
			break
		}
		end += i
		spans = append(spans, Span{Start: i, End: i + 1, Kind: TokenPunctuation})
		contentEnd := end
		for contentEnd > i+1 && line[contentEnd-1] == '/' {
			contentEnd--
		}
		if contentEnd > i+1 {
			nameEnd := i + 1
			for nameEnd < contentEnd && (line[nameEnd] == ' ' || line[nameEnd] == '\t' || line[nameEnd] == '/' || line[nameEnd] == '>') {
				nameEnd++
			}
			for nameEnd < contentEnd && line[nameEnd] != ' ' && line[nameEnd] != '\t' && line[nameEnd] != '/' {
				nameEnd++
			}
			if nameEnd > i+1 {
				spans = append(spans, Span{Start: i + 1, End: nameEnd, Kind: TokenTag})
			}
		}
		spans = append(spans, Span{Start: end, End: end + 1, Kind: TokenPunctuation})
		i = end + 1
	}
	return spans
}

func parseCSSLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if strings.HasPrefix(trimmed, "/*") {
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	if colon := strings.Index(trimmed, ":"); colon >= 0 {
		return []Span{{Start: indent, End: indent + colon, Kind: TokenProperty}}
	}
	return nil
}

func parseShellLine(line string) []Span {
	var spans []Span
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '#':
			return append(spans, Span{Start: i, End: len(line), Kind: TokenComment})
		case line[i] == '$':
			j := i + 1
			for j < len(line) && (line[j] == '_' || line[j] >= 'a' && line[j] <= 'z' || line[j] >= 'A' && line[j] <= 'Z' || line[j] >= '0' && line[j] <= '9') {
				j++
			}
			if j > i+1 {
				spans = append(spans, Span{Start: i, End: j, Kind: TokenVariable})
			}
			i = j - 1
		case line[i] == '-' && i+1 < len(line) && line[i+1] == '-':
			spans = append(spans, Span{Start: i, End: i + 2, Kind: TokenAttribute})
		case line[i] == '-' && i+1 < len(line) && (line[i+1] == '_' || line[i+1] >= 'a' && line[i+1] <= 'z'):
			j := i + 1
			for j < len(line) && (line[j] == '_' || line[j] == '-' || line[j] >= 'a' && line[j] <= 'z' || line[j] >= 'A' && line[j] <= 'Z' || line[j] >= '0' && line[j] <= '9') {
				j++
			}
			spans = append(spans, Span{Start: i, End: j, Kind: TokenAttribute})
			i = j - 1
		}
	}
	return spans
}

func parseSQLLine(line string) []Span {
	if idx := strings.Index(line, "--"); idx >= 0 {
		return []Span{{Start: idx, End: len(line), Kind: TokenComment}}
	}
	return nil
}

func parseDockerfileLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "#") {
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	if end := strings.IndexAny(trimmed, " \t"); end > 0 {
		word := trimmed[:end]
		if strings.EqualFold(word, "FROM") || strings.EqualFold(word, "RUN") || strings.EqualFold(word, "CMD") || strings.EqualFold(word, "ENTRYPOINT") || strings.EqualFold(word, "COPY") || strings.EqualFold(word, "ADD") || strings.EqualFold(word, "ENV") || strings.EqualFold(word, "ARG") || strings.EqualFold(word, "EXPOSE") || strings.EqualFold(word, "VOLUME") || strings.EqualFold(word, "USER") || strings.EqualFold(word, "WORKDIR") {
			return []Span{{Start: indent, End: indent + end, Kind: TokenKeyword}}
		}
	}
	return nil
}

func parseBuildFileLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "#") {
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	if strings.HasPrefix(trimmed, "=") {
		return []Span{{Start: indent, End: len(line), Kind: TokenVariable}}
	}
	if colon := strings.Index(trimmed, ":"); colon > 0 && !strings.Contains(trimmed[:colon], "=") {
		return []Span{{Start: indent, End: indent + colon, Kind: TokenFunction}, {Start: indent + colon, End: indent + colon + 1, Kind: TokenPunctuation}}
	}
	if strings.HasPrefix(trimmed, "$(") {
		return []Span{{Start: indent, End: indent + 2, Kind: TokenOperator}}
	}
	return nil
}

func parseEnvLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "#") {
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	if eq := strings.Index(trimmed, "="); eq > 0 {
		spans := []Span{{Start: indent, End: indent + eq, Kind: TokenProperty}}
		if value := indent + eq + 1; value < len(line) {
			spans = append(spans, Span{Start: value, End: len(line), Kind: TokenString})
		}
		return spans
	}
	return nil
}

func parseGitignoreLine(line string) []Span {
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "#") {
		return []Span{{Start: indent, End: len(line), Kind: TokenComment}}
	}
	end := len(line)
	if comment := strings.Index(trimmed, " #"); comment >= 0 {
		end = indent + comment
	}
	if end > indent {
		return []Span{{Start: indent, End: end, Kind: TokenString}}
	}
	return nil
}

func parseLicenseLine(line string) []Span {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	if strings.HasPrefix(trimmed, "MIT License") || strings.HasPrefix(trimmed, "Apache License") || strings.EqualFold(trimmed, "Copyright") {
		return []Span{{Start: indent, End: len(line), Kind: TokenMarkupHeading}}
	}
	return nil
}
