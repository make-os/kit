package console

import (
	"github.com/c-bata/go-prompt"
)

var commonFunc = [][]string{
	{".help", "Print the help message"},
	{".exit", "Exit the console"},
}

var optionsSuggestions = []prompt.Suggest{
	{Text: ".exit", Description: "Exit the console"},
	{Text: ".help", Description: "Print the help message"},
}

func optionsCompleter(d prompt.Document) []prompt.Suggest {
	if words := d.GetWordBeforeCursor(); len(words) > 1 {
		return prompt.FilterHasPrefix(optionsSuggestions, words, false)
	}
	return nil
}

// CompleterManager manages suggestions
type CompleterManager struct {
	completers []prompt.Completer
}

// newCompleterManager creates a completer manager.
func newCompleterManager() *CompleterManager {
	sm := new(CompleterManager)
	sm.completers = append(sm.completers, optionsCompleter)
	return sm
}

// add adds completers
func (sm *CompleterManager) add(completers ...prompt.Completer) {
	sm.completers = append(sm.completers, completers...)
}

// completer finds suggestions from known completers
func (sm *CompleterManager) completer(d prompt.Document) (suggestions []prompt.Suggest) {
	for _, c := range sm.completers {
		if sugs := c(d); len(sugs) > 0 {
			suggestions = append(suggestions, sugs...)
		}
	}
	return
}
