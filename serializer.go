package bokchoy

// Serializer defines an interface to implement a serializer.
type Serializer interface {
	Dumps(interface{}) ([]byte, error)
	Loads([]byte, interface{}) error
}

// newSerializer initializes a new Serializer.
func newSerializer(cfg SerializerConfig) Serializer {
	switch cfg.Type {
	default:
		return JSONSerializer{}
	}
}
