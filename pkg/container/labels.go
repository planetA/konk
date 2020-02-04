package container

import "fmt"

type Labels interface {
	AddLabel(key string, value interface{}) error
	GetLabelUint(key string) (uint, error)
	GetLabelString(key string) (string, error)
	ToStringSlice() []string
}

type labels struct {
	data map[string]interface{}
}

func NewLabels() Labels {
	return &labels{
		data: make(map[string]interface{}),
	}
}

func (l *labels) AddLabel(key string, value interface{}) error {
	if _, ok := l.data[key]; ok != false {
		return fmt.Errorf("Duplicate entry: %v", key)
	}

	l.data[key] = value
	return nil
}

func (l *labels) GetLabelUint(key string) (uint, error) {
	val, ok := l.data[key]
	if ok != true {
		return 0, fmt.Errorf("Key %v not found", key)
	}

	valUint, ok := val.(uint)
	if ok != true {
		return 0, fmt.Errorf("Type assertion failed at key %v", key)
	}

	return valUint, nil
}

func (l *labels) GetLabelString(key string) (string, error) {
	val, ok := l.data[key]
	if ok != true {
		return "", fmt.Errorf("Key %v not found", key)
	}

	valString, ok := val.(string)
	if ok != true {
		return "", fmt.Errorf("Type assertion failed at key %v", key)
	}

	return valString, nil
}

func (l *labels) ToStringSlice() []string {
	slice := make([]string, 0, len(l.data))

	for k, v := range l.data {
		slice = append(slice, fmt.Sprintf("konk-%v=%v", k, v))
	}

	return slice
}
