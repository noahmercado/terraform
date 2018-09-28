package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/backend"
	"github.com/hashicorp/terraform/configs/configload"
	"github.com/hashicorp/terraform/configs/configschema"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/states/statemgr"
	"github.com/hashicorp/terraform/terraform"
)

func TestLocal_applyBasic(t *testing.T) {
	b, cleanup := TestLocal(t)
	defer cleanup()
	p := TestLocalProvider(t, b, "test", applyFixtureSchema())

	p.ApplyResourceChangeResponse = providers.ApplyResourceChangeResponse{NewState: cty.ObjectVal(map[string]cty.Value{
		"id":  cty.StringVal("yes"),
		"ami": cty.StringVal("bar"),
	})}

	op, configCleanup := testOperationApply(t, "./test-fixtures/apply")
	defer configCleanup()

	run, err := b.Operation(context.Background(), op)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	<-run.Done()
	if run.Result != backend.OperationSuccess {
		t.Fatal("operation failed")
	}

	if p.ReadResourceCalled {
		t.Fatal("ReadResource should not be called")
	}

	if !p.PlanResourceChangeCalled {
		t.Fatal("diff should be called")
	}

	if !p.ApplyResourceChangeCalled {
		t.Fatal("apply should be called")
	}

	checkState(t, b.StateOutPath, `
test_instance.foo:
  ID = yes
  provider = provider.test
  ami = bar
`)
}

func TestLocal_applyEmptyDir(t *testing.T) {
	b, cleanup := TestLocal(t)
	defer cleanup()

	p := TestLocalProvider(t, b, "test", &terraform.ProviderSchema{})
	p.ApplyResourceChangeResponse = providers.ApplyResourceChangeResponse{NewState: cty.ObjectVal(map[string]cty.Value{"id": cty.StringVal("yes")})}

	op, configCleanup := testOperationApply(t, "./test-fixtures/empty")
	defer configCleanup()

	run, err := b.Operation(context.Background(), op)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	<-run.Done()
	if run.Result == backend.OperationSuccess {
		t.Fatal("operation succeeded; want error")
	}

	if p.ApplyResourceChangeCalled {
		t.Fatal("apply should not be called")
	}

	if _, err := os.Stat(b.StateOutPath); err == nil {
		t.Fatal("should not exist")
	}
}

func TestLocal_applyEmptyDirDestroy(t *testing.T) {
	b, cleanup := TestLocal(t)
	defer cleanup()
	p := TestLocalProvider(t, b, "test", &terraform.ProviderSchema{})

	p.ApplyResourceChangeResponse = providers.ApplyResourceChangeResponse{}

	op, configCleanup := testOperationApply(t, "./test-fixtures/empty")
	defer configCleanup()
	op.Destroy = true

	run, err := b.Operation(context.Background(), op)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	<-run.Done()
	if run.Result != backend.OperationSuccess {
		t.Fatalf("apply operation failed")
	}

	if p.ApplyResourceChangeCalled {
		t.Fatal("apply should not be called")
	}

	checkState(t, b.StateOutPath, `<no state>`)
}

func TestLocal_applyError(t *testing.T) {
	b, cleanup := TestLocal(t)
	defer cleanup()
	p := TestLocalProvider(t, b, "test", nil)

	var lock sync.Mutex
	errored := false
	p.GetSchemaReturn = &terraform.ProviderSchema{
		ResourceTypes: map[string]*configschema.Block{
			"test_instance": {
				Attributes: map[string]*configschema.Attribute{
					"ami":   {Type: cty.String, Optional: true},
					"error": {Type: cty.String, Optional: true},
				},
			},
		},
	}
	p.ApplyFn = func(
		info *terraform.InstanceInfo,
		s *terraform.InstanceState,
		d *terraform.InstanceDiff) (*terraform.InstanceState, error) {
		lock.Lock()
		defer lock.Unlock()

		if !errored && info.Id == "test_instance.bar" {
			errored = true
			return nil, fmt.Errorf("error")
		}

		return &terraform.InstanceState{ID: "foo"}, nil
	}
	p.DiffFn = func(
		*terraform.InstanceInfo,
		*terraform.InstanceState,
		*terraform.ResourceConfig) (*terraform.InstanceDiff, error) {
		return &terraform.InstanceDiff{
			Attributes: map[string]*terraform.ResourceAttrDiff{
				"ami": &terraform.ResourceAttrDiff{
					New: "bar",
				},
			},
		}, nil
	}

	op, configCleanup := testOperationApply(t, "./test-fixtures/apply-error")
	defer configCleanup()

	run, err := b.Operation(context.Background(), op)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	<-run.Done()
	if run.Result == backend.OperationSuccess {
		t.Fatal("operation succeeded; want failure")
	}

	checkState(t, b.StateOutPath, `
test_instance.foo:
  ID = foo
  provider = provider.test
	`)
}

func TestLocal_applyBackendFail(t *testing.T) {
	op, configCleanup := testOperationApply(t, "./test-fixtures/apply")
	defer configCleanup()

	b, cleanup := TestLocal(t)
	defer cleanup()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory")
	}
	err = os.Chdir(filepath.Dir(b.StatePath))
	if err != nil {
		t.Fatalf("failed to set temporary working directory")
	}
	defer os.Chdir(wd)

	b.Backend = &backendWithFailingState{}
	b.CLI = new(cli.MockUi)
	p := TestLocalProvider(t, b, "test", applyFixtureSchema())

	p.ApplyResourceChangeResponse = providers.ApplyResourceChangeResponse{NewState: cty.ObjectVal(map[string]cty.Value{"id": cty.StringVal("yes")})}

	run, err := b.Operation(context.Background(), op)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	<-run.Done()
	if run.Result == backend.OperationSuccess {
		t.Fatalf("apply succeeded; want error")
	}

	msgStr := b.CLI.(*cli.MockUi).ErrorWriter.String()
	if !strings.Contains(msgStr, "Failed to save state: fake failure") {
		t.Fatalf("missing \"fake failure\" message in output:\n%s", msgStr)
	}

	// The fallback behavior should've created a file errored.tfstate in the
	// current working directory.
	checkState(t, "errored.tfstate", `
test_instance.foo:
  ID = yes
  provider = provider.test
	`)
}

type backendWithFailingState struct {
	Local
}

func (b *backendWithFailingState) StateMgr(name string) (statemgr.Full, error) {
	return &failingState{
		statemgr.NewFilesystem("failing-state.tfstate"),
	}, nil
}

type failingState struct {
	*statemgr.Filesystem
}

func (s failingState) WriteState(state *states.State) error {
	return errors.New("fake failure")
}

func testOperationApply(t *testing.T, configDir string) (*backend.Operation, func()) {
	t.Helper()

	_, configLoader, configCleanup := configload.MustLoadConfigForTests(t, configDir)

	return &backend.Operation{
		Type:         backend.OperationTypeApply,
		ConfigDir:    configDir,
		ConfigLoader: configLoader,
	}, configCleanup
}

// testApplyState is just a common state that we use for testing refresh.
func testApplyState() *terraform.State {
	return &terraform.State{
		Version: 2,
		Modules: []*terraform.ModuleState{
			&terraform.ModuleState{
				Path: []string{"root"},
				Resources: map[string]*terraform.ResourceState{
					"test_instance.foo": &terraform.ResourceState{
						Type: "test_instance",
						Primary: &terraform.InstanceState{
							ID: "bar",
						},
					},
				},
			},
		},
	}
}

// applyFixtureSchema returns a schema suitable for processing the
// configuration in test-fixtures/apply . This schema should be
// assigned to a mock provider named "test".
func applyFixtureSchema() *terraform.ProviderSchema {
	return &terraform.ProviderSchema{
		ResourceTypes: map[string]*configschema.Block{
			"test_instance": {
				Attributes: map[string]*configschema.Attribute{
					"ami": {Type: cty.String, Optional: true},
					"id":  {Type: cty.String, Computed: true},
				},
			},
		},
	}
}
