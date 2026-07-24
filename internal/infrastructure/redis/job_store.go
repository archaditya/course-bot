package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"archadilm/internal/domain/entities"
	goredis "github.com/redis/go-redis/v9"
)

type JobStore struct {
	client *goredis.Client
}

func NewJobStore(client *goredis.Client) *JobStore {
	return &JobStore{client: client}
}

func (s *JobStore) SetJob(ctx context.Context, job *entities.Job) error {
	key := fmt.Sprintf("job:%s", job.ID)
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	
	return s.client.Set(ctx, key, data, 24*time.Hour).Err()
}

func (s *JobStore) GetJob(ctx context.Context, jobID string) (*entities.Job, error) {
	key := fmt.Sprintf("job:%s", jobID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	
	var job entities.Job
	err = json.Unmarshal(data, &job)
	return &job, err
}

func (s *JobStore) UpdateJobStatus(ctx context.Context, jobID string, status entities.JobStatus) error {
	key := fmt.Sprintf("job:%s", jobID)
	return s.client.HSet(ctx, key, "status", string(status)).Err()
}