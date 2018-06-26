package terraform

import (
	"testing"

	"github.com/hashicorp/terraform/config/configschema"
)

func TestMockResourceProvider_impl(t *testing.T) {
	var _ ResourceProvider = new(MockResourceProvider)
	var _ ResourceProviderCloser = new(MockResourceProvider)
}

// testProviderComponentFactory creates a componentFactory that contains only
// a single given.
func testProviderComponentFactory(name string, provider ResourceProvider) *basicComponentFactory {
	return &basicComponentFactory{
		providers: map[string]ResourceProviderFactory{
			name: ResourceProviderFactoryFixed(provider),
		},
	}
}

// mockProviderWithConfigSchema is a test helper to concisely create a mock
// provider with the given schema for its own configuration.
func mockProviderWithConfigSchema(schema *configschema.Block) *MockResourceProvider {
	return &MockResourceProvider{
		GetSchemaReturn: &ProviderSchema{
			Provider: schema,
		},
	}
}

// mockProviderWithResourceTypeSchema is a test helper to concisely create a mock
// provider with a schema containing a single resource type.
func mockProviderWithResourceTypeSchema(name string, schema *configschema.Block) *MockResourceProvider {
	return &MockResourceProvider{
		GetSchemaReturn: &ProviderSchema{
			ResourceTypes: map[string]*configschema.Block{
				name: schema,
			},
		},
	}
}

// mockProviderWithDataSourceSchema is a test helper to concisely create a mock
// provider with a schema containing a single data source.
func mockProviderWithDataSourceSchema(name string, schema *configschema.Block) *MockResourceProvider {
	return &MockResourceProvider{
		GetSchemaReturn: &ProviderSchema{
			DataSources: map[string]*configschema.Block{
				name: schema,
			},
		},
	}
}

// simpleMockProvider returns a MockResourceProvider that is pre-configured
// with schema for its own config, for a resource type called "test_object" and
// for a data source also called "test_object".
//
// All three schemas have the same content as returned by function
// simpleTestSchema.
//
// For most reasonable uses the returned provider must be registered in a
// componentFactory under the name "test". Use simpleMockComponentFactory
// to obtain a pre-configured componentFactory containing the result of
// this function along with simpleMockProvisioner, both registered as "test".
//
// The returned provider has no other behaviors by default, but the caller may
// modify it in order to stub any other required functionality, or modify
// the default schema stored in the field GetSchemaReturn. Each new call to
// simpleTestProvider produces entirely new instances of all of the nested
// objects so that callers can mutate without affecting mock objects.
func simpleMockProvider() *MockResourceProvider {
	return &MockResourceProvider{
		GetSchemaReturn: &ProviderSchema{
			Provider: simpleTestSchema(),
			ResourceTypes: map[string]*configschema.Block{
				"test_object": simpleTestSchema(),
			},
			DataSources: map[string]*configschema.Block{
				"test_object": simpleTestSchema(),
			},
		},
	}
}
