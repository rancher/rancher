package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PersistedState is the on-disk snapshot used to resume after restarts
type PersistedState struct {
    Version  string                         `json:"version"`
    Clusters map[string]PersistedCluster    `json:"clusters"` // key = cluster name
}

type PersistedCluster struct {
    Name           string    `json:"name"`
    ClusterID      string    `json:"cluster_id"`
    KWOKName       string    `json:"kwok_name,omitempty"`
    Port           int       `json:"port,omitempty"`
    KubeconfigPath string    `json:"kubeconfig_path,omitempty"`
    Status         string    `json:"status,omitempty"`
    CreatedAt      time.Time `json:"created_at,omitempty"`
}

func (a *ScaleAgent) stateFilePath() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("home dir: %w", err)
    }
    dir := filepath.Join(home, ".scale-cluster-agent")
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return "", fmt.Errorf("mkdir state dir: %w", err)
    }
    return filepath.Join(dir, "state.json"), nil
}

// SaveState writes the current clusters and KWOK mapping to disk
func (a *ScaleAgent) SaveState() error {
    path, err := a.stateFilePath()
    if err != nil {
        return err
    }

    st := PersistedState{Version: version, Clusters: map[string]PersistedCluster{}}
    for name, ci := range a.clusters {
        if name == "template" {
            continue
        }
        pc := PersistedCluster{
            Name:      name,
            ClusterID: ci.ClusterID,
            Status:    ci.Status,
        }
        if a.kwokManager != nil && ci.ClusterID != "" {
            if kc, ok := a.kwokManager.GetCluster(ci.ClusterID); ok && kc != nil {
                pc.KWOKName = kc.Name
                pc.Port = kc.Port
                // Persist path instead of full kubeconfig content
                pc.KubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kwok", "clusters", kc.Name, "kubeconfig.yaml")
                pc.CreatedAt = kc.CreatedAt
            }
        }
        st.Clusters[name] = pc
    }

    data, err := json.MarshalIndent(st, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal state: %w", err)
    }
    // atomic write
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return fmt.Errorf("write tmp state: %w", err)
    }
    if err := os.Rename(tmp, path); err != nil {
        return fmt.Errorf("rename state: %w", err)
    }
    return nil
}

// LoadState loads clusters from disk and restores KWOK mappings where possible
func (a *ScaleAgent) LoadState() error {
    path, err := a.stateFilePath()
    if err != nil {
        return err
    }
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil // nothing to load
        }
        return fmt.Errorf("read state: %w", err)
    }
    var st PersistedState
    if err := json.Unmarshal(data, &st); err != nil {
        return fmt.Errorf("parse state: %w", err)
    }

    if a.clusters == nil {
        a.clusters = map[string]*ClusterInfo{}
    }

    for name, pc := range st.Clusters {
        // Recreate ClusterInfo (minimal fields sufficient to reconnect)
        if _, exists := a.clusters[name]; !exists {
            a.clusters[name] = &ClusterInfo{Name: name, ClusterID: pc.ClusterID, Status: pc.Status}
        } else {
            // update fields in case they differ
            a.clusters[name].ClusterID = pc.ClusterID
            a.clusters[name].Status = pc.Status
        }

        // Recreate KWOK cluster record in manager if missing
        if a.kwokManager != nil && pc.ClusterID != "" {
            if _, ok := a.kwokManager.GetCluster(pc.ClusterID); !ok {
                // try restore by persisted name first; fallback to scan
                if _, err := a.kwokManager.RestoreClusterRecord(pc.ClusterID, pc.KWOKName); err != nil {
                    // not fatal; agent can recreate later if needed
                }
            }
        }
    }
    return nil
}
