package bokchoy

import "encoding/json"

type JSONSerializer struct {
}

func (s JSONSerializer) Dumps(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (s JSONSerializer) Loads(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (s JSONSerializer) String() string {
	return "json"
}

var _ Serializer = (*JSONSerializer)(nil)
