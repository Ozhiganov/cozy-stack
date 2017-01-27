package permissions

import (
	"encoding/json"
	"strings"
)

const verbSep = ","
const allVerbs = "ALL"
const allVerbsLength = 5

// Verb is one of GET,POST,PUT,PATCH,DELETE
type Verb string

// All possible Verbs, a subset of http methods
const (
	GET    = Verb("GET")
	POST   = Verb("POST")
	PUT    = Verb("PUT")
	PATCH  = Verb("PATCH")
	DELETE = Verb("DELETE")
)

var allVerbsOrder = []Verb{GET, POST, PUT, PATCH, DELETE}

// VerbSet is a Set of Verbs
type VerbSet map[Verb]struct{}

// Contains check if VerbSet contains a Verb
func (vs VerbSet) Contains(v Verb) bool {
	if len(vs) == 0 {
		return true // empty set = ALL
	}
	_, has := vs[v]
	return has
}

func (vs VerbSet) String() string {
	out := ""
	if len(vs) == 0 || len(vs) == allVerbsLength {
		return allVerbs
	}
	for _, v := range allVerbsOrder {
		if _, has := vs[v]; has {
			out += verbSep + string(v)
		}
	}
	return out[1:]
}

// MarshalJSON implements json.Marshaller on VerbSet
// the VerbSet is converted to a json array
func (vs VerbSet) MarshalJSON() ([]byte, error) {
	s := make([]string, len(vs))
	i := 0
	for _, v := range allVerbsOrder {
		if _, has := vs[v]; has {
			s[i] = string(v)
			i++
		}
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaller on VerbSet
// it expects a json array
func (vs *VerbSet) UnmarshalJSON(b []byte) error {
	if *vs == nil {
		*vs = make(VerbSet)
	}
	var s []string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	for v := range ALL {
		delete(*vs, v)
	}
	for v := range *vs {
		(*vs)[v] = struct{}{}
	}
	return nil
}

// VerbSplit parse a string into a VerbSet
func VerbSplit(in string) VerbSet {
	if in == allVerbs {
		return ALL
	}
	verbs := strings.Split(in, verbSep)
	out := make(VerbSet, len(verbs))
	for _, v := range verbs {
		out[Verb(v)] = struct{}{}
	}
	return out
}

// Verbs is a utility function to create VerbSets
func Verbs(verbs ...Verb) VerbSet {
	vs := make(VerbSet, len(verbs))
	for _, v := range verbs {
		vs[v] = struct{}{}
	}
	return vs
}

// ALL : the default VerbSet allows all Verbs
var ALL = Verbs(GET, POST, PUT, PATCH, DELETE)