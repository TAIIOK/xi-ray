package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CLISetPhase sets phase in state.json (used by panel-updater.sh).
func CLISetPhase(home, phase string) error {
	layout := LayoutForHome(home, filepath.Join(home, "panel.json"))
	store := NewStateStore(layout.StatePath)
	st, err := store.Load()
	if err != nil {
		return err
	}
	st.Phase = Phase(phase)
	return store.Save(st)
}

// CLIGetPhase prints current phase to stdout.
func CLIGetPhase(home string) error {
	layout := LayoutForHome(home, filepath.Join(home, "panel.json"))
	store := NewStateStore(layout.StatePath)
	st, err := store.Load()
	if err != nil {
		return err
	}
	fmt.Println(st.Phase)
	return nil
}

// CLIUpdaterApply swaps staged bundle into place.
func CLIUpdaterApply(home string) error {
	layout := LayoutForHome(home, filepath.Join(home, "panel.json"))
	store := NewStateStore(layout.StatePath)
	st, err := store.Load()
	if err != nil {
		return err
	}
	if st.Phase != PhaseVerified && st.Phase != PhaseApplying {
		return fmt.Errorf("cannot apply in phase %s", st.Phase)
	}
	st.Phase = PhaseApplying
	if err := store.Save(st); err != nil {
		return err
	}
	return applyBundle(layout, store)
}

func applyBundle(layout Layout, store *StateStore) error {
	stagingPanel := filepath.Join(layout.StagingDir, "panel")
	if _, err := os.Stat(stagingPanel); err != nil {
		return fmt.Errorf("staging panel missing: %w", err)
	}
	if err := backupPanel(layout); err != nil {
		return err
	}
	if err := atomicReplace(stagingPanel, layout.PanelBin); err != nil {
		return err
	}
	if err := installScriptsFromStaging(layout); err != nil {
		return err
	}
	st, _ := store.Load()
	st.Phase = PhaseRestarting
	return store.Save(st)
}

func backupPanel(layout Layout) error {
	if _, err := os.Stat(layout.PanelBin); err != nil {
		return nil
	}
	return copyFile(layout.PanelBin, layout.PanelPrevious)
}

func atomicReplace(src, dest string) error {
	newPath := dest + ".new"
	if err := copyFile(src, newPath); err != nil {
		return err
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		return err
	}
	return os.Rename(newPath, dest)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func installScriptsFromStaging(layout Layout) error {
	srcDir := filepath.Join(layout.StagingDir, "scripts")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(layout.ScriptsDir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(layout.ScriptsDir); err == nil {
		_ = os.RemoveAll(layout.ScriptsPrevDir)
		_ = copyDir(layout.ScriptsDir, layout.ScriptsPrevDir)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dest := filepath.Join(layout.ScriptsDir, e.Name())
		if err := copyFile(src, dest); err != nil {
			return err
		}
		_ = os.Chmod(dest, 0o755)
		legacy := filepath.Join(layout.Home, e.Name())
		_ = copyFile(src, legacy)
		_ = os.Chmod(legacy, 0o755)
	}
	return nil
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dest, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(s, d); err != nil {
			return err
		}
	}
	return nil
}

// CLIRollback restores panel.previous.
func CLIRollback(home string) error {
	layout := LayoutForHome(home, filepath.Join(home, "panel.json"))
	store := NewStateStore(layout.StatePath)
	if _, err := os.Stat(layout.PanelPrevious); err != nil {
		return fmt.Errorf("panel.previous not found")
	}
	if _, err := os.Stat(layout.PanelBin); err == nil {
		failed := layout.PanelBin + ".failed"
		_ = os.Rename(layout.PanelBin, failed)
	}
	if err := copyFile(layout.PanelPrevious, layout.PanelBin); err != nil {
		return err
	}
	if _, err := os.Stat(layout.ScriptsPrevDir); err == nil {
		_ = os.RemoveAll(layout.ScriptsDir)
		_ = copyDir(layout.ScriptsPrevDir, layout.ScriptsDir)
	}
	st, _ := store.Load()
	st.Phase = PhaseRestarting
	return store.Save(st)
}
