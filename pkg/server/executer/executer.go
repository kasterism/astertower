package executer

import (
	"github.com/kasterism/astertower/pkg/server/parser"
)

type WorkflowStarter struct {
	Workflow_id string
	dag         *parser.NodeSet
}

// Constructor of the workflow initiator
func NewWorkflowStarter(workflow_id string, dag *parser.NodeSet) *WorkflowStarter {
	return &WorkflowStarter{
		Workflow_id: workflow_id,
		dag:         dag,
	}
}

// Enter the information for the DAG and turn the map into a real workflow
func (w *WorkflowStarter) CreateWorkflow() error {
	// TODO: Execute workflow

	return nil
}
