package provider

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// snapshotActionConfig: a project and a terraform_data trigger. Creating the
// trigger creates snapshot `snap`; changing its input (step 2) restores
// snapshot `restore`. terraform_data keeps the trigger outside GNS3, so a
// restore cannot drift any resource in the plan.
func snapshotActionConfig(project, input, snap, restore string) string {
	return fmt.Sprintf(`
resource "gns3_project" "test" {
  name = %q
}

action "gns3_snapshot" "create" {
  config {
    project_id = gns3_project.test.project_id
    name       = %q
  }
}

action "gns3_snapshot" "restore" {
  config {
    project_id = gns3_project.test.project_id
    name       = %q
    restore    = true
  }
}

resource "terraform_data" "trigger" {
  input = %q

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.gns3_snapshot.create]
    }
    action_trigger {
      events  = [after_update]
      actions = [action.gns3_snapshot.restore]
    }
  }
}
`, project, snap, restore, input)
}

// checkSnapshots checks the project's snapshot names on the server (sorted).
func (e testEnv) checkSnapshots(want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		pid := s.RootModule().Resources["gns3_project.test"].Primary.Attributes["project_id"]
		var got []string
		if e.fake != nil {
			got = e.fake.snapshotNames(pid)
		} else {
			snaps, err := e.client.ListSnapshots(context.Background(), pid)
			if err != nil {
				return err
			}
			for _, sn := range snaps {
				got = append(got, sn.Name)
			}
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			return fmt.Errorf("snapshots on server are %q, want %q", got, want)
		}
		return nil
	}
}

// checkRestored checks which snapshots the fake restored. A live server keeps
// no record of restores, so against one it checks nothing.
func (e testEnv) checkRestored(want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if e.fake == nil {
			return nil
		}
		pid := s.RootModule().Resources["gns3_project.test"].Primary.Attributes["project_id"]
		if got := e.fake.restoredSnapshots(pid); !slices.Equal(got, want) {
			return fmt.Errorf("restored snapshots are %q, want %q", got, want)
		}
		return nil
	}
}

func snapshotActionTestCase(env testEnv) resource.TestCase {
	project := acctest.RandomWithPrefix("tf-acc-snapshot")
	return resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		CheckDestroy:             env.checkGone("gns3_project"),
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + snapshotActionConfig(project, "one", "snap1", "snap1"),
				Check:  resource.ComposeAggregateTestCheckFunc(env.checkSnapshots("snap1"), env.checkRestored()),
			},
			{
				Config: env.providerBlock + snapshotActionConfig(project, "two", "snap1", "snap1"),
				Check:  resource.ComposeAggregateTestCheckFunc(env.checkSnapshots("snap1"), env.checkRestored("snap1")),
			},
			{
				// Restoring a name nobody created fails.
				Config:      env.providerBlock + snapshotActionConfig(project, "three", "snap1", "nope"),
				ExpectError: regexp.MustCompile(`no snapshot named "nope"`),
			},
		},
	}
}

func TestSnapshotAction(t *testing.T) { resource.UnitTest(t, snapshotActionTestCase(unitEnv(t))) }

// TestSnapshotActionAmbiguous: a restore refuses to pick between two
// snapshots with the same name. Unit only: the docs don't say whether a
// server accepts a duplicate snapshot name, and no live server has been probed.
func TestSnapshotActionAmbiguous(t *testing.T) {
	env := unitEnv(t)
	project := acctest.RandomWithPrefix("tf-unit-snapshot")
	var projectID string
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0)},
		Steps: []resource.TestStep{
			{
				Config: env.providerBlock + snapshotActionConfig(project, "one", "snap1", "snap1"),
				Check: func(s *terraform.State) error {
					projectID = s.RootModule().Resources["gns3_project.test"].Primary.Attributes["project_id"]
					return nil
				},
			},
			{
				// The fake answers 409 to a duplicate name through the API, so
				// the second "snap1" goes straight into its state.
				PreConfig: func() {
					f := env.fake
					f.mu.Lock()
					defer f.mu.Unlock()
					f.snapshots[projectID] = append(f.snapshots[projectID], map[string]any{
						"snapshot_id": "00000000-0000-4000-8000-000000000001", "project_id": projectID,
						"name": "snap1", "created_at": int64(0),
					})
				},
				Config:      env.providerBlock + snapshotActionConfig(project, "two", "snap1", "snap1"),
				ExpectError: regexp.MustCompile(`has 2 snapshots named "snap1"`),
			},
		},
	})
}

func TestAccSnapshotAction(t *testing.T) { resource.Test(t, snapshotActionTestCase(accEnv(t))) }
