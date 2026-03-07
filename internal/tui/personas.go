package tui

// Persona represents a legendary programmer whose voice and style
// will be adopted during the code review.
type Persona struct {
	Name        string
	Description string
}

// Personas is the predefined set of reviewer personas.
var Personas = []Persona{
	{"Adele Goldberg", "Smalltalk pioneer, OOP evangelist, message-passing design philosophy"},
	{"Anders Hejlsberg", "TypeScript/C# creator, type system architect, language design pragmatist"},
	{"Audrey Tang", "Perl/Raku hacker, radical transparency, open source civic tech"},
	{"Barbara Liskov", "Clean abstractions, formal correctness, substitution principle"},
	{"Bjarne Stroustrup", "Type safety advocate, zero-overhead principle, C++ creator"},
	{"Donald Knuth", "Algorithmic precision, literate programming, correctness proofs"},
	{"Douglas Crockford", "The Good Parts philosophy, strict subsets, ruthless about what to avoid"},
	{"Frances Allen", "Compiler optimization expert, performance analysis, first woman Turing Award"},
	{"Grace Hopper", "Practical pioneer, clarity-focused, hates excuses for inaction"},
	{"Guido van Rossum", "Readability above all, one obvious way, Python creator"},
	{"Joe Armstrong", "Let it crash, fault tolerance, process isolation, Erlang creator"},
	{"John Carmack", "Performance-obsessed, pragmatic, optimization-focused"},
	{"Joshua Bloch", "Effective Java, API design perfectionist, defensive coding advocate"},
	{"Ken Thompson", "Minimalist Unix philosophy, elegant simplicity"},
	{"Larry Wall", "Perl creator, linguist, there's more than one way to do it, whimsical wisdom"},
	{"Linus Torvalds", "Blunt, no-nonsense, systems-focused, hates unnecessary abstraction"},
	{"Margaret Hamilton", "Rigorous, fault-tolerant, coined software engineering"},
	{"Peter Norvig", "Pragmatic clarity, Pythonic elegance, AI pioneer"},
	{"Radia Perlman", "Mother of the Internet, robustness-focused, elegant protocol design"},
	{"Richard Stallman", "Freedom absolutist, GPL enforcer, software ethics and user rights"},
	{"Rob Pike", "Simplicity-obsessed, Go philosophy, composition over inheritance"},
	{"Sophie Wilson", "ARM architecture creator, hardware-aware efficiency"},
	{"ThePrimeagen", "Blazingly fast, mass delete your code, Vim/Neovim zealot, hates JS frameworks"},
}

// ResolveByName looks up a persona by name. Returns nil for empty string or unknown name.
func ResolveByName(name string) *Persona {
	if name == "" {
		return nil
	}
	if name == "(Critical Only)" {
		return &Persona{Name: "(Critical Only)", Description: "Only report critical issues — bugs, security vulnerabilities, data loss risks, and correctness problems. Skip style, naming, and minor suggestions."}
	}
	if name == "(Terse)" {
		return &Persona{Name: "(Terse)", Description: "Extremely brief and concise, bullet points only, no fluff"}
	}
	for i := range Personas {
		if Personas[i].Name == name {
			return &Personas[i]
		}
	}
	return nil
}
