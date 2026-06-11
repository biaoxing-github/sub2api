//go:build unit

package service

import "context"

// testPtrFloat64 returns a pointer to the given float64 value.
func testPtrFloat64(v float64) *float64 { return &v }

// testPtrInt returns a pointer to the given int value.
func testPtrInt(v int) *int { return &v }

// testPtrString returns a pointer to the given string value.
func testPtrString(v string) *string { return &v }

// testPtrBool returns a pointer to the given bool value.
func testPtrBool(v bool) *bool { return &v }

// settingRepoStub is a test stub for SettingRepository
type settingRepoStub struct {
	values map[string]string
	err    error
}

func (s *settingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *settingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			result[key] = v
		}
	}
	return result, nil
}

func (s *settingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}
