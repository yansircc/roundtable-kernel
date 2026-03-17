package rtk

import (
	"path/filepath"
	"testing"
)

func TestPreferredUIRootPrefersOverride(t *testing.T) {
	t.Parallel()

	root := "/workspace"
	override := "/bundle/ui/dist"
	result := preferredUIRoot(root, "/skill/scripts/rtk-darwin-arm64", override, func(path string) bool {
		return false
	})
	if result != override {
		t.Fatalf("unexpected ui root: %s", result)
	}
}

func TestPreferredUIRootPrefersWorkspaceAssets(t *testing.T) {
	t.Parallel()

	root := "/workspace"
	workspaceUI := filepath.Join(root, "ui", "dist")
	result := preferredUIRoot(root, "/skill/scripts/rtk-darwin-arm64", "", func(path string) bool {
		return path == workspaceUI
	})
	if result != workspaceUI {
		t.Fatalf("unexpected ui root: %s", result)
	}
}

func TestPreferredUIRootFallsBackToBundledAssets(t *testing.T) {
	t.Parallel()

	root := "/workspace"
	executable := "/skill/scripts/rtk-darwin-arm64"
	bundledUI := filepath.Join("/skill/scripts", "..", "ui", "dist")
	result := preferredUIRoot(root, executable, "", func(path string) bool {
		return filepath.Clean(path) == filepath.Clean(bundledUI)
	})
	if filepath.Clean(result) != filepath.Clean(bundledUI) {
		t.Fatalf("unexpected ui root: %s", result)
	}
}

func TestPreferredUIRootFallsBackToWorkspacePathWhenAssetsMissing(t *testing.T) {
	t.Parallel()

	root := "/workspace"
	workspaceUI := filepath.Join(root, "ui", "dist")
	result := preferredUIRoot(root, "/skill/scripts/rtk-darwin-arm64", "", func(path string) bool {
		return false
	})
	if result != workspaceUI {
		t.Fatalf("unexpected ui root: %s", result)
	}
}
