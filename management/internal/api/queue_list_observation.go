package api

import (
	"github.com/chennqqi/rabbit-jetstream/internal/topology"
	"github.com/chennqqi/rabbit-jetstream/management/internal/jetstream"
)

type queueListObservation struct {
	State     string   `json:"state"`
	Stream    string   `json:"stream,omitempty"`
	Messages  *uint64  `json:"messages,omitempty"`
	Consumers *int     `json:"consumers,omitempty"`
	Reasons   []string `json:"reasons"`
}

type queueListItem struct {
	topology.Declaration
	Observation queueListObservation `json:"observation"`
}

func observeQueueList(declarations []topology.Declaration, streams []jetstream.Stream, streamErr error) []queueListItem {
	byName := make(map[string][]jetstream.Stream, len(streams))
	for _, stream := range streams {
		byName[stream.Name] = append(byName[stream.Name], stream)
	}
	result := make([]queueListItem, 0, len(declarations))
	for _, declaration := range declarations {
		observation := queueListObservation{State: "unavailable", Reasons: []string{}}
		plan := declaration.Plan
		if declaration.Queue == "" || plan.Queue != declaration.Queue || plan.Revision != declaration.Revision || plan.Stream.Name == "" {
			observation.Reasons = append(observation.Reasons, "declaration_inconsistent")
		} else {
			observation.Stream = plan.Stream.Name
			switch matches := byName[plan.Stream.Name]; {
			case streamErr != nil:
				observation.Reasons = append(observation.Reasons, "stream_collection_unavailable")
			case len(matches) == 0:
				observation.State = "missing"
				observation.Reasons = append(observation.Reasons, "stream_missing")
			case len(matches) > 1:
				observation.Reasons = append(observation.Reasons, "stream_identity_ambiguous")
			default:
				stream := matches[0]
				messages, consumers := stream.Messages, stream.Consumers
				observation.Messages, observation.Consumers = &messages, &consumers
				observation.State = "present"
				if !streamMatchesPlan(stream, declaration) {
					observation.State = "degraded"
					observation.Reasons = append(observation.Reasons, "stream_configuration_mismatch")
				}
				if !streamReplicaEvidenceCurrent(stream, plan.Stream.Replicas) {
					observation.State = "degraded"
					observation.Reasons = append(observation.Reasons, "stream_replicas_not_current")
				}
			}
		}
		result = append(result, queueListItem{Declaration: declaration, Observation: observation})
	}
	return result
}

func streamMatchesPlan(stream jetstream.Stream, declaration topology.Declaration) bool {
	observed := &topology.ObservedStream{
		Name: stream.Name, Subjects: stream.Subjects, Storage: stream.Storage, Replicas: stream.Replicas,
		Retention: stream.Retention, Discard: stream.Discard, MaxAgeNanos: stream.MaxAgeNanos,
		MaxBytes: stream.MaxBytes, MaxMessages: stream.MaxMessages, Metadata: stream.Metadata,
	}
	result := topology.Reconcile(declaration.Plan, topology.ObservedTopology{Stream: observed})
	return len(result.Operations) > 0 && result.Operations[0].Resource == "stream" && result.Operations[0].Action == "noop"
}

func streamReplicaEvidenceCurrent(stream jetstream.Stream, desired int) bool {
	if desired < 1 || stream.Replicas != desired {
		return false
	}
	if desired == 1 {
		return true
	}
	if stream.Cluster == nil || stream.Cluster.Leader == "" || len(stream.Cluster.Replicas) != desired-1 {
		return false
	}
	seen := map[string]bool{stream.Cluster.Leader: true}
	for _, replica := range stream.Cluster.Replicas {
		if replica.Name == "" || seen[replica.Name] || !replica.Current || replica.Offline {
			return false
		}
		seen[replica.Name] = true
	}
	return true
}
