package domain

import "testing"

func TestSubstituteSkillArguments(t *testing.T) {
	tests := []struct {
		name, body, arguments, want string
	}{
		{
			name:      "deployment example",
			body:      "Deploy to $0 in region $1. Full request was: $ARGUMENTS",
			arguments: "staging eu-west",
			want:      "Deploy to staging in region eu-west. Full request was: staging eu-west",
		},
		{
			name:      "mixed repeated placeholders",
			body:      "$ARGUMENTS[0]/$0/$ARGUMENTS[1]/$1/$ARGUMENTS/$ARGUMENTS/$0",
			arguments: "staging eu-west",
			want:      "staging/staging/eu-west/eu-west/staging eu-west/staging eu-west/staging",
		},
		{
			name:      "multi digit indexes",
			body:      "$10/$ARGUMENTS[10]/$1/$ARGUMENTS[11]",
			arguments: "a b c d e f g h i j k l",
			want:      "k/k/b/l",
		},
		{
			name:      "whitespace and body formatting",
			body:      "  $0\n\n$ARGUMENTS[1]\n$ARGUMENTS  ",
			arguments: " \tstaging\t \neu-west\r\n ",
			want:      "  staging\n\neu-west\nstaging\t \neu-west  ",
		},
		{name: "no arguments", body: "[$ARGUMENTS][$0][$ARGUMENTS[1]]", want: "[][][]"},
		{name: "missing argument", body: "$0/$1/$ARGUMENTS[2]", arguments: "staging", want: "staging//"},
		{name: "index overflow", body: "[$999999999999999999999999999999][$ARGUMENTS[999999999999999999999999999999]]", arguments: "staging", want: "[][]"},
		{
			name:      "argument placeholders remain literal",
			body:      "$0|$1|$ARGUMENTS[2]|$ARGUMENTS",
			arguments: "$1 $ARGUMENTS[0] $ARGUMENTS",
			want:      "$1|$ARGUMENTS[0]|$ARGUMENTS|$1 $ARGUMENTS[0] $ARGUMENTS",
		},
		{name: "no placeholders", body: "Deploy blueberry.", arguments: "staging", want: "Deploy blueberry."},
		{name: "quotes are literal", body: "$0|$1", arguments: "\"staging eu-west\"", want: "\"staging|eu-west\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubstituteSkillArguments(tt.body, tt.arguments); got != tt.want {
				t.Fatalf("substituteSkillArguments(%q, %q) = %q, want %q", tt.body, tt.arguments, got, tt.want)
			}
		})
	}
}

func TestValidSkillName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"apple", true}, {"apple-2_test", true}, {"", false}, {"../apple", false}, {"apple/banana", false}, {"apple space", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidSkillName(tt.name); got != tt.want {
				t.Fatalf("validSkillName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
