package coordinator

import (
	"strings"
	"testing"
)

func TestDefaultDefinitionRoutesProjectQuestionsToHelper(t *testing.T) {
	def := DefaultDefinition()
	if !strings.Contains(def.Instruction, "派给 fkteams_helper") {
		t.Fatal("coordinator prompt does not route project questions to fkteams_helper")
	}
}
