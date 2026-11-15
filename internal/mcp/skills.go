package mcp

import (
	"fmt"
	"strings"

	embedded "github.com/bethropolis/sift/skills"
)

func promptDefinitions() []Prompt {
	var prompts []Prompt
	for _, s := range embedded.BundledSkills {
		prompts = append(prompts, Prompt{
			Name:        s.Name,
			Description: s.Description,
		})
	}
	return prompts
}

func resourceDefinitions() []Resource {
	var resources []Resource
	for _, s := range embedded.BundledSkills {
		resources = append(resources, Resource{
			URI:         "skill://" + s.Name,
			Name:        s.Name,
			Description: s.Description,
			MIMEType:    "text/markdown",
		})
	}
	return resources
}

func handleGetPrompt(name string) (*PromptGetResult, error) {
	skill, ok := embedded.FindSkill(name)
	if !ok {
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}

	content, err := embedded.ReadSkill(skill.Name)
	if err != nil {
		return nil, err
	}

	return &PromptGetResult{
		Description: skill.Description,
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: ContentBlock{
					Type: "text",
					Text: string(content),
				},
			},
		},
	}, nil
}

func handleReadResource(uri string) (*ResourceReadResult, error) {
	const prefix = "skill://"
	if !strings.HasPrefix(uri, prefix) {
		return nil, fmt.Errorf("unsupported resource URI scheme: %s", uri)
	}
	skillName := strings.TrimPrefix(uri, prefix)

	content, err := embedded.ReadSkill(skillName)
	if err != nil {
		return nil, err
	}

	return &ResourceReadResult{
		Contents: []ResourceContent{
			{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     string(content),
			},
		},
	}, nil
}
