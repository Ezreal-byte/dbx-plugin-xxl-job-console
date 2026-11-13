package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type manifestMcpContribution struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	ExternalTools bool   `json:"external_tools"`
	Toolsets      []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"toolsets"`
	Tools map[string]struct {
		Toolset string `json:"toolset"`
		Access  string `json:"access"`
	} `json:"tools"`
}

func TestManifestDeclaresEveryMcpToolAccess(t *testing.T) {
	data, err := os.ReadFile("../manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID            string                    `json:"id"`
		Version       string                    `json:"version"`
		Contributions []manifestMcpContribution `json:"contributions"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ID != pluginID {
		t.Fatalf("manifest id %q does not match plugin id %q", manifest.ID, pluginID)
	}
	if manifest.Version != version {
		t.Fatalf("manifest version %q does not match sidecar version %q", manifest.Version, version)
	}
	var mcp *manifestMcpContribution
	for i := range manifest.Contributions {
		if manifest.Contributions[i].Type == "mcp" {
			mcp = &manifest.Contributions[i]
			break
		}
	}
	if mcp == nil {
		t.Fatal("manifest has no mcp contribution")
	}
	if !mcp.ExternalTools || mcp.ID == "" {
		t.Fatalf("mcp contribution is not externally declared: %+v", *mcp)
	}
	toolsets := map[string]bool{}
	for _, toolset := range mcp.Toolsets {
		if toolsets[toolset.ID] {
			t.Fatalf("duplicate toolset %q", toolset.ID)
		}
		if strings.TrimSpace(toolset.Title) == "" {
			t.Fatalf("toolset %q has no title", toolset.ID)
		}
		toolsets[toolset.ID] = true
	}
	catalog := toolCatalog()
	if len(mcp.Tools) != len(catalog) {
		t.Fatalf("manifest declares %d tools, backend catalog has %d", len(mcp.Tools), len(catalog))
	}
	for _, tool := range catalog {
		declaration, ok := mcp.Tools[tool.Name]
		if !ok {
			t.Fatalf("tool %q is missing an mcp declaration", tool.Name)
		}
		wantAccess := "write"
		if tool.Group == "query" {
			wantAccess = "read"
		} else if tool.Group == "delete" || strings.HasPrefix(tool.Name, "delete_") {
			wantAccess = "full"
		}
		if declaration.Access != wantAccess {
			t.Fatalf("tool %q declares access %q, want %q", tool.Name, declaration.Access, wantAccess)
		}
		if declaration.Toolset != tool.Group {
			t.Fatalf("tool %q declares toolset %q, want %q", tool.Name, declaration.Toolset, tool.Group)
		}
		if !toolsets[declaration.Toolset] {
			t.Fatalf("tool %q references undeclared toolset %q", tool.Name, declaration.Toolset)
		}
	}
	for name := range mcp.Tools {
		if _, ok := findTool(name); !ok {
			t.Fatalf("manifest declares unknown tool %q", name)
		}
	}
}
