package cluster

import "context"

type Summary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     bool   `json:"default"`
}

type Source interface {
	ListClusters(ctx context.Context) ([]Summary, error)
}

type Service struct {
	Source Source
}

func (s Service) List(ctx context.Context) ([]Summary, error) {
	if s.Source == nil {
		return []Summary{defaultCluster()}, nil
	}

	items, err := s.Source.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []Summary{defaultCluster()}, nil
	}
	return items, nil
}

func defaultCluster() Summary {
	return Summary{
		ID:          "default",
		Name:        "Default Cluster",
		Description: "기본 클러스터 타겟",
		Default:     true,
	}
}
