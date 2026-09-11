package payload

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// PayloadType represents the detected structured format of a payload.
type PayloadType int

const (
	None PayloadType = iota
	JSON
	XML
	YAML
	Logfmt
)

func (t PayloadType) String() string {
	switch t {
	case JSON:
		return "JSON"
	case XML:
		return "XML"
	case YAML:
		return "YAML"
	case Logfmt:
		return "LOGFMT"
	default:
		return ""
	}
}

// ColorPalette defines the colors used for syntax beautification.
type ColorPalette struct {
	Cyan   lipgloss.Color
	Yellow lipgloss.Color
	Green  lipgloss.Color
	Purple lipgloss.Color
	Accent lipgloss.Color
	Muted  lipgloss.Color
	Fg     lipgloss.Color
}

// Detection encapsulates the result of payload inspection.
type Detection struct {
	Type           PayloadType
	TypeLabel      string // e.g. "JSON", "XML", "YAML", "LOGFMT"
	Prefix         string // any text before the structured payload
	RawPayload     string // original raw payload substring
	Suffix         string // any text after the structured payload
	FormattedText  string // clean indented multi-line representation
	ColorizedLines []string
}

// DetectAndFormat inspects a string for JSON, XML, YAML, or Logfmt structured data,
// formats it with indentation, and colorizes it according to the provided color palette.
func DetectAndFormat(s string, p ColorPalette) Detection {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 {
		return Detection{Type: None}
	}

	// 1. Check for JSON (highest precision and common in modern logs)
	jsonDet := DetectAndFormatJSON(s)
	if jsonDet.HasJSON {
		colorized := ColorizeJSON(jsonDet.IndentedJSON, p)
		return Detection{
			Type:           JSON,
			TypeLabel:      "JSON",
			Prefix:         jsonDet.Prefix,
			RawPayload:     jsonDet.RawJSON,
			Suffix:         jsonDet.Suffix,
			FormattedText:  jsonDet.IndentedJSON,
			ColorizedLines: strings.Split(colorized, "\n"),
		}
	}

	// 2. Check for XML
	xmlDet, ok := detectAndFormatXML(s, p)
	if ok {
		return xmlDet
	}

	// 3. Check for YAML
	yamlDet, ok := detectAndFormatYAML(s, p)
	if ok {
		return yamlDet
	}

	// 4. Check for Logfmt (key=value pairs)
	logfmtDet, ok := detectAndFormatLogfmt(s, p)
	if ok {
		return logfmtDet
	}

	return Detection{Type: None}
}

// ── JSON Formatting ───────────────────────────────────────────────────────────

// JSONDetection contains the results of inspecting a string for JSON payloads.
type JSONDetection struct {
	HasJSON      bool
	Prefix       string
	RawJSON      string
	Suffix       string
	IndentedJSON string
}

// DetectAndFormatJSON inspects a string to see if it is a JSON object/array
// or contains an embedded JSON object/array.
func DetectAndFormatJSON(s string) JSONDetection {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 {
		return JSONDetection{HasJSON: false}
	}

	// Direct JSON (object or array)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		if json.Valid([]byte(trimmed)) {
			var out bytes.Buffer
			if err := json.Indent(&out, []byte(trimmed), "", "  "); err == nil {
				return JSONDetection{
					HasJSON:      true,
					RawJSON:      trimmed,
					IndentedJSON: out.String(),
				}
			}
		}
	}

	// Embedded JSON in a prefixed log line
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '{' && ch != '[' {
			continue
		}
		closeChar := byte('}')
		if ch == '[' {
			closeChar = ']'
		}
		lastIdx := strings.LastIndexByte(s, closeChar)
		for lastIdx > i {
			candidate := s[i : lastIdx+1]
			if json.Valid([]byte(candidate)) {
				var out bytes.Buffer
				if err := json.Indent(&out, []byte(candidate), "", "  "); err == nil {
					return JSONDetection{
						HasJSON:      true,
						Prefix:       strings.TrimSpace(s[:i]),
						RawJSON:      candidate,
						Suffix:       strings.TrimSpace(s[lastIdx+1:]),
						IndentedJSON: out.String(),
					}
				}
			}
			lastIdx = strings.LastIndexByte(s[:lastIdx], closeChar)
		}
	}

	return JSONDetection{HasJSON: false}
}

// ColorizeJSON applies theme colors to an indented JSON string.
func ColorizeJSON(indentedJSON string, p ColorPalette) string {
	styleKey := lipgloss.NewStyle().Foreground(p.Cyan).Bold(true)
	styleString := lipgloss.NewStyle().Foreground(p.Green)
	styleNumber := lipgloss.NewStyle().Foreground(p.Yellow)
	styleBoolNull := lipgloss.NewStyle().Foreground(p.Purple)
	styleBrace := lipgloss.NewStyle().Foreground(p.Accent)
	stylePunct := lipgloss.NewStyle().Foreground(p.Muted)

	var sb strings.Builder
	runes := []rune(indentedJSON)
	n := len(runes)
	i := 0

	for i < n {
		ch := runes[i]

		if unicode.IsSpace(ch) {
			sb.WriteRune(ch)
			i++
			continue
		}

		if ch == '"' {
			start := i
			i++
			for i < n {
				if runes[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if runes[i] == '"' {
					i++
					break
				}
				i++
			}
			strLiteral := string(runes[start:i])

			lookahead := i
			for lookahead < n && unicode.IsSpace(runes[lookahead]) {
				lookahead++
			}
			if lookahead < n && runes[lookahead] == ':' {
				sb.WriteString(styleKey.Render(strLiteral))
			} else {
				sb.WriteString(styleString.Render(strLiteral))
			}
			continue
		}

		if ch == '{' || ch == '}' || ch == '[' || ch == ']' {
			sb.WriteString(styleBrace.Render(string(ch)))
			i++
			continue
		}

		if ch == ':' || ch == ',' {
			sb.WriteString(stylePunct.Render(string(ch)))
			i++
			continue
		}

		if unicode.IsDigit(ch) || ch == '-' {
			start := i
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.' || runes[i] == 'e' || runes[i] == 'E' || runes[i] == '+' || runes[i] == '-') {
				i++
			}
			numStr := string(runes[start:i])
			sb.WriteString(styleNumber.Render(numStr))
			continue
		}

		if unicode.IsLetter(ch) {
			start := i
			for i < n && unicode.IsLetter(runes[i]) {
				i++
			}
			ident := string(runes[start:i])
			if ident == "true" || ident == "false" || ident == "null" {
				sb.WriteString(styleBoolNull.Render(ident))
			} else {
				sb.WriteString(ident)
			}
			continue
		}

		sb.WriteRune(ch)
		i++
	}

	return sb.String()
}

// ── XML Formatting ────────────────────────────────────────────────────────────

func detectAndFormatXML(s string, p ColorPalette) (Detection, bool) {
	startIdx := strings.IndexByte(s, '<')
	lastIdx := strings.LastIndexByte(s, '>')
	if startIdx < 0 || lastIdx <= startIdx+2 {
		return Detection{}, false
	}

	candidate := strings.TrimSpace(s[startIdx : lastIdx+1])
	if len(candidate) < 3 || candidate[0] != '<' || candidate[len(candidate)-1] != '>' {
		return Detection{}, false
	}

	if candidate[1] == ' ' || candidate[1] == '\t' || candidate[1] == '\n' {
		return Detection{}, false
	}

	decoder := xml.NewDecoder(strings.NewReader(candidate))
	var tokens []xml.Token
	elementCount := 0
	openElements := 0

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Detection{}, false
		}
		switch tok.(type) {
		case xml.StartElement:
			elementCount++
			openElements++
		case xml.EndElement:
			openElements--
		}
		tokens = append(tokens, xml.CopyToken(tok))
	}

	if elementCount == 0 || openElements != 0 {
		return Detection{}, false
	}

	formatted, colorized := formatAndColorizeXMLTokens(tokens, p)
	if len(formatted) == 0 {
		return Detection{}, false
	}

	return Detection{
		Type:           XML,
		TypeLabel:      "XML",
		Prefix:         strings.TrimSpace(s[:startIdx]),
		RawPayload:     candidate,
		Suffix:         strings.TrimSpace(s[lastIdx+1:]),
		FormattedText:  formatted,
		ColorizedLines: colorized,
	}, true
}

func formatAndColorizeXMLTokens(tokens []xml.Token, p ColorPalette) (string, []string) {
	styleTag := lipgloss.NewStyle().Foreground(p.Cyan).Bold(true)
	styleAttrKey := lipgloss.NewStyle().Foreground(p.Yellow)
	styleAttrVal := lipgloss.NewStyle().Foreground(p.Green)
	styleBracket := lipgloss.NewStyle().Foreground(p.Muted)
	styleText := lipgloss.NewStyle().Foreground(p.Fg)
	styleComment := lipgloss.NewStyle().Foreground(p.Muted).Italic(true)

	var plainLines []string
	var styledLines []string

	indent := 0
	getIndent := func() string {
		return strings.Repeat("  ", indent)
	}

	renderStartTag := func(el xml.StartElement, selfClosing bool) (string, string) {
		var plain strings.Builder
		var styled strings.Builder

		plain.WriteString("<")
		plain.WriteString(el.Name.Local)

		styled.WriteString(styleBracket.Render("<"))
		styled.WriteString(styleTag.Render(el.Name.Local))

		for _, attr := range el.Attr {
			plain.WriteString(fmt.Sprintf(" %s=\"%s\"", attr.Name.Local, attr.Value))

			styled.WriteString(" ")
			styled.WriteString(styleAttrKey.Render(attr.Name.Local))
			styled.WriteString(styleBracket.Render("=\""))
			styled.WriteString(styleAttrVal.Render(attr.Value))
			styled.WriteString(styleBracket.Render("\""))
		}

		if selfClosing {
			plain.WriteString("/>")
			styled.WriteString(styleBracket.Render("/>"))
		} else {
			plain.WriteString(">")
			styled.WriteString(styleBracket.Render(">"))
		}
		return plain.String(), styled.String()
	}

	renderEndTag := func(name string) (string, string) {
		plain := fmt.Sprintf("</%s>", name)
		styled := styleBracket.Render("</") + styleTag.Render(name) + styleBracket.Render(">")
		return plain, styled
	}

	n := len(tokens)
	i := 0
	for i < n {
		tok := tokens[i]
		switch el := tok.(type) {
		case xml.StartElement:
			pad := getIndent()

			if i+1 < n {
				if endEl, ok := tokens[i+1].(xml.EndElement); ok && endEl.Name.Local == el.Name.Local {
					plainStart, styledStart := renderStartTag(el, true)
					plainLines = append(plainLines, pad+plainStart)
					styledLines = append(styledLines, pad+styledStart)
					i += 2
					continue
				}
			}

			if i+2 < n {
				charTok, isChar := tokens[i+1].(xml.CharData)
				endEl, isEnd := tokens[i+2].(xml.EndElement)
				if isChar && isEnd && endEl.Name.Local == el.Name.Local {
					text := strings.TrimSpace(string(charTok))
					plainStart, styledStart := renderStartTag(el, false)
					plainEnd, styledEnd := renderEndTag(el.Name.Local)
					plainLines = append(plainLines, fmt.Sprintf("%s%s%s%s", pad, plainStart, text, plainEnd))
					styledLines = append(styledLines, fmt.Sprintf("%s%s%s%s", pad, styledStart, styleText.Render(text), styledEnd))
					i += 3
					continue
				}
			}

			plainStart, styledStart := renderStartTag(el, false)
			plainLines = append(plainLines, pad+plainStart)
			styledLines = append(styledLines, pad+styledStart)
			indent++
			i++

		case xml.EndElement:
			if indent > 0 {
				indent--
			}
			pad := getIndent()
			plainEnd, styledEnd := renderEndTag(el.Name.Local)
			plainLines = append(plainLines, pad+plainEnd)
			styledLines = append(styledLines, pad+styledEnd)
			i++

		case xml.CharData:
			text := strings.TrimSpace(string(el))
			if len(text) > 0 {
				pad := getIndent()
				plainLines = append(plainLines, pad+text)
				styledLines = append(styledLines, pad+styleText.Render(text))
			}
			i++

		case xml.Comment:
			pad := getIndent()
			cmt := strings.TrimSpace(string(el))
			plainLines = append(plainLines, fmt.Sprintf("%s<!-- %s -->", pad, cmt))
			styledLines = append(styledLines, pad+styleComment.Render(fmt.Sprintf("<!-- %s -->", cmt)))
			i++

		default:
			i++
		}
	}

	return strings.Join(plainLines, "\n"), styledLines
}

// ── YAML Formatting ───────────────────────────────────────────────────────────

func tryParseYAML(str string) (*yaml.Node, bool) {
	trimmed := strings.TrimSpace(str)
	if !strings.Contains(trimmed, ": ") && !strings.Contains(trimmed, ":\n") && !strings.HasPrefix(trimmed, "- ") {
		return nil, false
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 && !strings.HasPrefix(trimmed, "- ") {
		return nil, false
	}

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(trimmed), &node); err != nil {
		return nil, false
	}

	if len(node.Content) == 0 {
		return nil, false
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode && root.Kind != yaml.SequenceNode {
		return nil, false
	}

	if root.Kind == yaml.MappingNode {
		if len(root.Content) < 4 {
			if len(root.Content) < 2 || (root.Content[1].Kind != yaml.MappingNode && root.Content[1].Kind != yaml.SequenceNode) {
				return nil, false
			}
		}
	}

	return root, true
}

func detectAndFormatYAML(s string, p ColorPalette) (Detection, bool) {
	candidate := s
	if !strings.Contains(candidate, "\n") && strings.Contains(candidate, `\n`) {
		candidate = strings.ReplaceAll(candidate, `\n`, "\n")
	}

	trimmed := strings.TrimSpace(candidate)

	// 1. Full string
	if root, ok := tryParseYAML(trimmed); ok {
		return buildYAMLDetection(root, "", trimmed, p)
	}

	// 2. Embedded YAML with log prefix
	newlineIdx := strings.IndexByte(candidate, '\n')
	if newlineIdx > 0 {
		firstLine := candidate[:newlineIdx]

		searchStart := 0
		for {
			colonIdx := strings.Index(firstLine[searchStart:], ": ")
			if colonIdx < 0 {
				break
			}
			absColonIdx := searchStart + colonIdx
			prefix := strings.TrimSpace(candidate[:absColonIdx+1])
			payloadSub := strings.TrimSpace(candidate[absColonIdx+2:])

			if root, ok := tryParseYAML(payloadSub); ok {
				return buildYAMLDetection(root, prefix, payloadSub, p)
			}
			searchStart = absColonIdx + 2
		}

		prefix := strings.TrimSpace(firstLine)
		payloadSub := strings.TrimSpace(candidate[newlineIdx+1:])
		if root, ok := tryParseYAML(payloadSub); ok {
			return buildYAMLDetection(root, prefix, payloadSub, p)
		}
	}

	return Detection{}, false
}

func buildYAMLDetection(root *yaml.Node, prefix, rawPayload string, p ColorPalette) (Detection, bool) {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return Detection{}, false
	}
	_ = enc.Close()

	formatted := strings.TrimRight(out.String(), "\n")
	if len(formatted) == 0 {
		return Detection{}, false
	}

	colorized := colorizeYAML(formatted, p)

	return Detection{
		Type:           YAML,
		TypeLabel:      "YAML",
		Prefix:         prefix,
		RawPayload:     rawPayload,
		Suffix:         "",
		FormattedText:  formatted,
		ColorizedLines: colorized,
	}, true
}

func colorizeYAML(yamlStr string, p ColorPalette) []string {
	styleKey := lipgloss.NewStyle().Foreground(p.Cyan).Bold(true)
	styleString := lipgloss.NewStyle().Foreground(p.Green)
	styleNumber := lipgloss.NewStyle().Foreground(p.Yellow)
	styleBoolNull := lipgloss.NewStyle().Foreground(p.Purple)
	styleBullet := lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	styleColon := lipgloss.NewStyle().Foreground(p.Muted)

	var lines []string
	rawLines := strings.Split(yamlStr, "\n")

	for _, line := range rawLines {
		if len(line) == 0 {
			lines = append(lines, "")
			continue
		}

		trimmed := strings.TrimLeft(line, " ")
		leadSpaces := line[:len(line)-len(trimmed)]

		var sb strings.Builder
		sb.WriteString(leadSpaces)

		if strings.HasPrefix(trimmed, "- ") {
			sb.WriteString(styleBullet.Render("- "))
			trimmed = trimmed[2:]
		}

		colonIdx := strings.Index(trimmed, ": ")
		if colonIdx >= 0 {
			keyPart := trimmed[:colonIdx]
			valPart := strings.TrimSpace(trimmed[colonIdx+2:])

			sb.WriteString(styleKey.Render(keyPart))
			sb.WriteString(styleColon.Render(": "))

			if valPart == "true" || valPart == "false" || valPart == "null" || valPart == "~" {
				sb.WriteString(styleBoolNull.Render(valPart))
			} else if _, err := strconv.ParseFloat(valPart, 64); err == nil {
				sb.WriteString(styleNumber.Render(valPart))
			} else {
				sb.WriteString(styleString.Render(valPart))
			}
		} else if strings.HasSuffix(trimmed, ":") {
			keyPart := trimmed[:len(trimmed)-1]
			sb.WriteString(styleKey.Render(keyPart))
			sb.WriteString(styleColon.Render(":"))
		} else {
			if trimmed == "true" || trimmed == "false" || trimmed == "null" {
				sb.WriteString(styleBoolNull.Render(trimmed))
			} else if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
				sb.WriteString(styleNumber.Render(trimmed))
			} else {
				sb.WriteString(styleString.Render(trimmed))
			}
		}

		lines = append(lines, sb.String())
	}

	return lines
}

// ── Logfmt / Key-Value Formatting ─────────────────────────────────────────────

type kvPair struct {
	key string
	val string
}

type parsedLogfmt struct {
	pairs     []kvPair
	startByte int
	endByte   int
}

func detectAndFormatLogfmt(s string, p ColorPalette) (Detection, bool) {
	trimmed := strings.TrimSpace(s)
	if strings.Count(trimmed, "=") < 2 {
		return Detection{}, false
	}

	res := parseLogfmtStrict(s)
	if len(res.pairs) < 2 {
		return Detection{}, false
	}

	maxKeyLen := 0
	for _, kv := range res.pairs {
		if len(kv.key) > maxKeyLen {
			maxKeyLen = len(kv.key)
		}
	}
	if maxKeyLen > 24 {
		maxKeyLen = 24
	}

	styleKey := lipgloss.NewStyle().Foreground(p.Cyan).Bold(true)
	styleString := lipgloss.NewStyle().Foreground(p.Green)
	styleNumber := lipgloss.NewStyle().Foreground(p.Yellow)
	styleBool := lipgloss.NewStyle().Foreground(p.Purple)

	var plainLines []string
	var styledLines []string

	for _, kv := range res.pairs {
		padKey := padOrTrunc(kv.key, maxKeyLen)
		plainLines = append(plainLines, fmt.Sprintf("%s = %s", padKey, kv.val))

		var valStyled string
		lowerVal := strings.ToLower(kv.val)
		if lowerVal == "true" || lowerVal == "false" || lowerVal == "null" {
			valStyled = styleBool.Render(kv.val)
		} else if _, err := strconv.ParseFloat(kv.val, 64); err == nil {
			valStyled = styleNumber.Render(kv.val)
		} else {
			valStyled = styleString.Render(kv.val)
		}

		styledLines = append(styledLines, fmt.Sprintf("  %s %s %s", styleKey.Render(padKey), lipgloss.NewStyle().Foreground(p.Muted).Render("="), valStyled))
	}

	prefix := ""
	if res.startByte > 0 {
		prefix = strings.TrimSpace(s[:res.startByte])
	}
	suffix := ""
	if res.endByte < len(s) {
		suffix = strings.TrimSpace(s[res.endByte:])
	}
	rawPayload := s[res.startByte:res.endByte]

	return Detection{
		Type:           Logfmt,
		TypeLabel:      "LOGFMT",
		Prefix:         prefix,
		RawPayload:     rawPayload,
		Suffix:         suffix,
		FormattedText:  strings.Join(plainLines, "\n"),
		ColorizedLines: styledLines,
	}, true
}

func parseLogfmtStrict(s string) parsedLogfmt {
	var pairs []kvPair
	runes := []rune(s)
	n := len(runes)
	i := 0
	firstStart := -1
	lastEnd := -1

	for i < n {
		for i < n && unicode.IsSpace(runes[i]) {
			i++
		}
		if i >= n {
			break
		}

		keyStart := i
		if !unicode.IsLetter(runes[i]) && runes[i] != '_' {
			for i < n && !unicode.IsSpace(runes[i]) {
				i++
			}
			continue
		}

		validKey := true
		for i < n && runes[i] != '=' && !unicode.IsSpace(runes[i]) {
			r := runes[i]
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.') {
				validKey = false
			}
			i++
		}

		if !validKey || i >= n || runes[i] != '=' {
			for i < n && !unicode.IsSpace(runes[i]) {
				i++
			}
			continue
		}

		key := string(runes[keyStart:i])
		i++

		if firstStart < 0 {
			firstStart = keyStart
		}

		if i >= n {
			pairs = append(pairs, kvPair{key: key, val: ""})
			lastEnd = i
			break
		}

		valStart := i
		var val string
		if runes[i] == '"' {
			i++
			valStart = i
			for i < n {
				if runes[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if runes[i] == '"' {
					val = string(runes[valStart:i])
					i++
					break
				}
				i++
			}
			if i >= n && val == "" {
				val = string(runes[valStart:])
			}
			lastEnd = i
		} else {
			for i < n && !unicode.IsSpace(runes[i]) {
				i++
			}
			val = string(runes[valStart:i])
			lastEnd = i
		}

		if len(key) > 0 {
			pairs = append(pairs, kvPair{key: key, val: val})
		}
	}

	startByte := 0
	endByte := len(s)
	if firstStart >= 0 {
		startByte = len(string(runes[:firstStart]))
	}
	if lastEnd >= 0 {
		endByte = len(string(runes[:lastEnd]))
	}

	return parsedLogfmt{
		pairs:     pairs,
		startByte: startByte,
		endByte:   endByte,
	}
}

func padOrTrunc(s string, width int) string {
	if len(s) > width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// WrapTextLines breaks multi-line text cleanly across maxWidth boundaries,
// respecting words when possible.
func WrapTextLines(text string, maxWidth int) []string {
	if maxWidth < 10 {
		maxWidth = 10
	}
	var result []string
	rawLines := strings.Split(text, "\n")

	for _, line := range rawLines {
		line = strings.TrimRight(line, "\r")
		if len([]rune(line)) <= maxWidth {
			result = append(result, line)
			continue
		}

		remaining := line
		for len([]rune(remaining)) > maxWidth {
			runes := []rune(remaining)
			breakIdx := -1
			for j := maxWidth; j >= 0; j-- {
				if j < len(runes) && unicode.IsSpace(runes[j]) {
					breakIdx = j
					break
				}
			}

			if breakIdx <= 0 {
				breakIdx = maxWidth
			}

			result = append(result, string(runes[:breakIdx]))
			remaining = strings.TrimLeft(string(runes[breakIdx:]), " ")
		}
		if len(remaining) > 0 {
			result = append(result, remaining)
		}
	}
	return result
}
