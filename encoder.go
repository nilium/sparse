package sparse

import "strings"

// valueEscaper attempts to escape most, but not all, values.
var valueEscaper = strings.NewReplacer(
	"#", `\#`,
	";", `\;`,
	"\b", `\b`,
	"\f", `\f`,
	"\n", "\\\n",
	"\r", `\r`,
	"\v", `\v`,
	"\x00", `\0`,
)

// keyEscaper includes all escape codes from valueEscaper, with the addition of whitespace and the bang.
var keyEscaper = strings.NewReplacer(
	" ", `\ `,
	"#", `\#`,
	";", `\;`,
	"\b", `\b`,
	"\f", `\f`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
	"\v", `\v`,
	"\x00", `\0`,
	"!", `\!`,
)
