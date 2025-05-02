package cueworkflow

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"cuelang.org/go/cue"
)

// ActionHandler defines the interface for handling different types of actions
type ActionHandler interface {
	// Execute executes the action and returns a result or error
	Execute(ctx context.Context, action cue.Value) (interface{}, error)
	// ActionType returns the type of action this handler can process
	ActionType() string
}

// WorkflowRunner is responsible for executing a workflow
type WorkflowRunner struct {
	// actionHandlers maps action types to their handlers
	actionHandlers map[string]ActionHandler
	// clusterConfig holds the cluster configuration
	clusterConfig *ClusterConfig
	// logger for logging workflow execution
	logger *log.Logger
}

// NewWorkflowRunner creates a new workflow runner with the given cluster configuration
func NewWorkflowRunner(clusterConfig *ClusterConfig, logger *log.Logger) *WorkflowRunner {
	return &WorkflowRunner{
		actionHandlers: make(map[string]ActionHandler),
		clusterConfig:  clusterConfig,
		logger:         logger,
	}
}

// RegisterActionHandler registers a handler for a specific action type
func (r *WorkflowRunner) RegisterActionHandler(handler ActionHandler) {
	r.actionHandlers[handler.ActionType()] = handler
}

// ExecuteWorkflow runs the specified workflow
func (r *WorkflowRunner) ExecuteWorkflow(ctx context.Context, workflow *cue.Value) error {
	// Extract the workflow name and description for logging
	name, _ := workflow.LookupPath(cue.ParsePath("name")).String()
	desc, _ := workflow.LookupPath(cue.ParsePath("description")).String()

	r.logger.Printf("Starting workflow: %s - %s", name, desc)

	// Get the stages
	stagesValue := workflow.LookupPath(cue.ParsePath("stages"))
	if !stagesValue.Exists() {
		return fmt.Errorf("workflow does not define stages")
	}

	// Iterate through each stage
	stagesIter, err := stagesValue.List()
	if err != nil {
		return fmt.Errorf("stages is not a list: %w", err)
	}

	stageIndex := 0
	for stagesIter.Next() {
		stage := stagesIter.Value()

		// Extract stage metadata
		stageName, _ := stage.LookupPath(cue.ParsePath("name")).String()
		stageDesc, _ := stage.LookupPath(cue.ParsePath("description")).String()

		r.logger.Printf("Executing stage %d: %s - %s", stageIndex+1, stageName, stageDesc)

		// Get the actions for this stage
		actionsValue := stage.LookupPath(cue.ParsePath("actions"))
		if !actionsValue.Exists() {
			r.logger.Printf("Stage %s has no actions, skipping", stageName)
			continue
		}

		// Iterate through each action
		actionsIter, err := actionsValue.List()
		if err != nil {
			return fmt.Errorf("actions is not a list in stage %s: %w", stageName, err)
		}

		actionIndex := 0
		for actionsIter.Next() {
			action := actionsIter.Value()

			// Execute the action
			if err := r.executeAction(ctx, actionIndex+1, action); err != nil {
				return fmt.Errorf("error executing action %d in stage %s: %w", actionIndex+1, stageName, err)
			}

			actionIndex++
		}

		stageIndex++
	}

	r.logger.Printf("Workflow completed successfully: %s", name)
	return nil
}

// executeAction executes a single action in the workflow
func (r *WorkflowRunner) executeAction(ctx context.Context, index int, action cue.Value) error {
	// Extract the action type
	actionTypeValue := action.LookupPath(cue.ParsePath("type"))
	if !actionTypeValue.Exists() {
		return fmt.Errorf("action does not define a type")
	}

	actionType, err := actionTypeValue.String()
	if err != nil {
		return fmt.Errorf("action type is not a string: %w", err)
	}

	// Find the handler for this action type
	var handler ActionHandler
	var handlerFound bool

	// Try exact match first
	handler, handlerFound = r.actionHandlers[actionType]

	// If not found, try prefix match (for handlers that handle multiple action types like BMC)
	if !handlerFound {
		for prefix, h := range r.actionHandlers {
			if prefix != "" && len(prefix) > 0 && prefix[len(prefix)-1] == ':' && strings.HasPrefix(actionType, prefix) {
				handler = h
				handlerFound = true
				break
			}
		}
	}

	if !handlerFound {
		return fmt.Errorf("no handler registered for action type: %s", actionType)
	}

	// Log the action execution
	r.logger.Printf("Executing action %d: %s", index, actionType)

	// Execute the action with the handler
	startTime := time.Now()
	result, err := handler.Execute(ctx, action)
	duration := time.Since(startTime)

	if err != nil {
		r.logger.Printf("Action %d failed after %v: %v", index, duration, err)
		return err
	}

	// Log the result
	r.logger.Printf("Action %d completed in %v with result: %v", index, duration, result)
	return nil
}

// DefaultHandlers returns a map of default action handlers
func DefaultHandlers() map[string]ActionHandler {
	handlers := make(map[string]ActionHandler)

	// Register the built-in handlers
	handlers["common:wait"] = &WaitActionHandler{}

	return handlers
}

// WaitActionHandler implements the ActionHandler interface for the wait action
type WaitActionHandler struct{}

func (h *WaitActionHandler) ActionType() string {
	return "common:wait"
}

func (h *WaitActionHandler) Execute(ctx context.Context, action cue.Value) (interface{}, error) {
	// Extract the wait duration from the parameters
	paramsValue := action.LookupPath(cue.ParsePath("params"))
	if !paramsValue.Exists() {
		return nil, fmt.Errorf("wait action does not define parameters")
	}

	secondsValue := paramsValue.LookupPath(cue.ParsePath("seconds"))
	if !secondsValue.Exists() {
		return nil, fmt.Errorf("wait parameters do not include 'seconds'")
	}

	seconds, err := secondsValue.Int64()
	if err != nil {
		return nil, fmt.Errorf("seconds parameter is not an integer: %w", err)
	}

	// Create a context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()

	// Wait for the specified duration or until the context is cancelled
	select {
	case <-time.After(time.Duration(seconds) * time.Second):
		return fmt.Sprintf("waited for %d seconds", seconds), nil
	case <-waitCtx.Done():
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("wait cancelled")
		}
		return nil, ctx.Err()
	}
}
