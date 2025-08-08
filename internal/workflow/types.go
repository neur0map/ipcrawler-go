package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlosm/ipcrawler/internal/template"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	ID               string `yaml:"id"`
	Description      string `yaml:"description"`
	Parallel         bool   `yaml:"parallel"`
	ContinueOnError  bool   `yaml:"continue_on_error,omitempty"` // Continue workflow even if steps fail
	Steps            []Step `yaml:"steps"`
	FolderPath       string `yaml:"-"` // Not serialized, set during loading
}

type Step struct {
	ID           string                 `yaml:"id"`
	Tool         string                 `yaml:"tool,omitempty"`
	Type         string                 `yaml:"type,omitempty"`
	UseFlags     string                 `yaml:"use_flags,omitempty"`
	OverrideArgs []string               `yaml:"override_args,omitempty"`
	Output       string                 `yaml:"output"`
	Inputs       []string               `yaml:"inputs,omitempty"`
	DependsOn    []string               `yaml:"depends_on,omitempty"`
	Optional     bool                   `yaml:"optional,omitempty"` // Step failure won't fail workflow
	ExtraData    map[string]interface{} `yaml:",inline"`
}

// LoadWorkflowsAutoDiscover automatically discovers and loads all workflow folders
func LoadWorkflowsAutoDiscover(target string) ([]Workflow, error) {
	folders, err := discoverWorkflowFolders()
	if err != nil {
		return nil, err
	}
	return LoadWorkflowsFromFolders(folders, target)
}

// discoverWorkflowFolders finds all subdirectories in the workflows/ directory
func discoverWorkflowFolders() ([]string, error) {
	workflowsDir := "workflows"
	if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workflows directory not found: %s", workflowsDir)
	}

	var folders []string
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return nil, fmt.Errorf("reading workflows directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			folderPath := filepath.Join(workflowsDir, entry.Name())
			// Check if this folder contains any .yaml files
			if hasWorkflowFiles(folderPath) {
				folders = append(folders, folderPath)
			}
		}
	}

	if len(folders) == 0 {
		return nil, fmt.Errorf("no workflow folders found in %s", workflowsDir)
	}

	return folders, nil
}

// hasWorkflowFiles checks if a directory contains any .yaml workflow files
func hasWorkflowFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			return true
		}
	}
	return false
}

func LoadWorkflowsFromFolders(folders []string, target string) ([]Workflow, error) {
	var workflows []Workflow
	templateData := map[string]string{"target": target}
	
	for _, folder := range folders {
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			continue
		}
		
		err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			
			if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return nil
			}
			
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading workflow file %s: %w", path, err)
			}
			
			var wf Workflow
			if err := yaml.Unmarshal(data, &wf); err != nil {
				return fmt.Errorf("parsing workflow file %s: %w", path, err)
			}
			
			if err := validateWorkflow(&wf); err != nil {
				return fmt.Errorf("validating workflow %s: %w", wf.ID, err)
			}
			
			applyTemplateToWorkflow(&wf, templateData)
			// Set the folder path for this workflow
			wf.FolderPath = folder
			workflows = append(workflows, wf)
			
			return nil
		})
		
		if err != nil {
			return nil, err
		}
	}
	
	return workflows, nil
}

func validateWorkflow(wf *Workflow) error {
	if wf.ID == "" {
		return fmt.Errorf("workflow missing ID")
	}
	
	stepIDs := make(map[string]bool)
	for i, step := range wf.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d missing ID", i)
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true
		
		for _, dep := range step.DependsOn {
			if !stepIDs[dep] && !willExist(wf.Steps, dep) {
				return fmt.Errorf("step %s depends on unknown step: %s", step.ID, dep)
			}
		}
	}
	
	if err := checkCircularDependencies(wf.Steps); err != nil {
		return err
	}
	
	return nil
}

func willExist(steps []Step, id string) bool {
	for _, step := range steps {
		if step.ID == id {
			return true
		}
	}
	return false
}

func checkCircularDependencies(steps []Step) error {
	adjList := make(map[string][]string)
	for _, step := range steps {
		adjList[step.ID] = step.DependsOn
	}
	
	visited := make(map[string]int)
	
	var dfs func(node string) error
	dfs = func(node string) error {
		if visited[node] == 1 {
			return fmt.Errorf("circular dependency detected involving step: %s", node)
		}
		if visited[node] == 2 {
			return nil
		}
		
		visited[node] = 1
		for _, dep := range adjList[node] {
			if err := dfs(dep); err != nil {
				return err
			}
		}
		visited[node] = 2
		
		return nil
	}
	
	for node := range adjList {
		if visited[node] == 0 {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}
	
	return nil
}

func applyTemplateToWorkflow(wf *Workflow, data map[string]string) {
	for i := range wf.Steps {
		step := &wf.Steps[i]
		step.Output = template.ApplyTemplate(step.Output, data)
		step.OverrideArgs = template.ApplyTemplateToSlice(step.OverrideArgs, data)
		step.Inputs = template.ApplyTemplateToSlice(step.Inputs, data)
		
		for key, val := range step.ExtraData {
			step.ExtraData[key] = template.ApplyTemplateToInterface(val, data)
		}
	}
}