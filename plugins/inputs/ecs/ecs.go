//go:generate ../../../tools/readme_config_includer/generator
package ecs

import (
	_ "embed"
	"os"
	"strings"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/inputs"
)

// DO NOT REMOVE THE NEXT TWO LINES! This is required to embed the sampleConfig data.
//
//go:embed sample.conf
var sampleConfig string

// Ecs config object
type Ecs struct {
	EndpointURL string `toml:"endpoint_url"`
	Timeout     config.Duration

	ContainerNameInclude []string `toml:"container_name_include"`
	ContainerNameExclude []string `toml:"container_name_exclude"`

	ContainerStatusInclude []string `toml:"container_status_include"`
	ContainerStatusExclude []string `toml:"container_status_exclude"`

	LabelInclude []string `toml:"ecs_label_include"`
	LabelExclude []string `toml:"ecs_label_exclude"`

	newClient func(timeout time.Duration, endpoint string, version int) (*EcsClient, error)

	client              Client
	filtersCreated      bool
	labelFilter         filter.Filter
	containerNameFilter filter.Filter
	statusFilter        filter.Filter
	metadataVersion     int
}

const (
	KB = 1000
	MB = 1000 * KB
	GB = 1000 * MB
	TB = 1000 * GB
	PB = 1000 * TB

	v2Endpoint = "http://169.254.170.2"
)

func (*Ecs) SampleConfig() string {
	return sampleConfig
}

// Gather is the entrypoint for telegraf metrics collection
func (ecs *Ecs) Gather(acc telegraf.Accumulator) error {
	err := initSetup(ecs)
	if err != nil {
		return err
	}

	task, stats, err := PollSync(ecs.client)
	if err != nil {
		return err
	}

	mergeTaskStats(task, stats)

	taskTags := map[string]string{
		"cluster":  task.Cluster,
		"task_arn": task.TaskARN,
		"family":   task.Family,
		"revision": task.Revision,
	}

	// accumulate metrics
	ecs.accTask(task, taskTags, acc)
	ecs.accContainers(task, taskTags, acc)

	return nil
}

func initSetup(ecs *Ecs) error {
	if ecs.client == nil {
		resolveEndpoint(ecs)

		c, err := ecs.newClient(time.Duration(ecs.Timeout), ecs.EndpointURL, ecs.metadataVersion)
		if err != nil {
			return err
		}
		ecs.client = c
	}

	// Create filters
	if !ecs.filtersCreated {
		err := ecs.createContainerNameFilters()
		if err != nil {
			return err
		}
		err = ecs.createContainerStatusFilters()
		if err != nil {
			return err
		}
		err = ecs.createLabelFilters()
		if err != nil {
			return err
		}
		ecs.filtersCreated = true
	}

	return nil
}

func resolveEndpoint(ecs *Ecs) {
	if ecs.EndpointURL != "" {
		// Use metadata v2 API since endpoint is set explicitly.
		ecs.metadataVersion = 2
		return
	}

	// Auto-detect metadata endpoint version.

	// Use metadata v3 if available.
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v3.html
	v3Endpoint := os.Getenv("ECS_CONTAINER_METADATA_URI")
	if v3Endpoint != "" {
		ecs.EndpointURL = v3Endpoint
		ecs.metadataVersion = 3
		return
	}

	// Use v2 endpoint if nothing else is available.
	ecs.EndpointURL = v2Endpoint
	ecs.metadataVersion = 2
}

func (ecs *Ecs) accTask(task *Task, tags map[string]string, acc telegraf.Accumulator) {
	taskFields := map[string]interface{}{
		"desired_status": task.DesiredStatus,
		"known_status":   task.KnownStatus,
		"limit_cpu":      task.Limits["CPU"],
		"limit_mem":      task.Limits["Memory"],
	}

	acc.AddFields("ecs_task", taskFields, tags)
}

func (ecs *Ecs) accContainers(task *Task, taskTags map[string]string, acc telegraf.Accumulator) {
	for _, c := range task.Containers {
		if !ecs.containerNameFilter.Match(c.Name) {
			continue
		}

		if !ecs.statusFilter.Match(strings.ToUpper(c.KnownStatus)) {
			continue
		}

		// add matching ECS container Labels
		containerTags := map[string]string{
			"id":   c.ID,
			"name": c.Name,
		}
		for k, v := range c.Labels {
			if ecs.labelFilter.Match(k) {
				containerTags[k] = v
			}
		}
		tags := mergeTags(taskTags, containerTags)

		parseContainerStats(c, acc, tags)
	}
}

// returns a new map with the same content values as the input map
func copyTags(in map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		out[k] = v
	}
	return out
}

// returns a new map with the merged content values of the two input maps
func mergeTags(a map[string]string, b map[string]string) map[string]string {
	c := copyTags(a)
	for k, v := range b {
		c[k] = v
	}
	return c
}

func (ecs *Ecs) createContainerNameFilters() error {
	containerNameFilter, err := filter.NewIncludeExcludeFilter(ecs.ContainerNameInclude, ecs.ContainerNameExclude)
	if err != nil {
		return err
	}
	ecs.containerNameFilter = containerNameFilter
	return nil
}

func (ecs *Ecs) createLabelFilters() error {
	labelFilter, err := filter.NewIncludeExcludeFilter(ecs.LabelInclude, ecs.LabelExclude)
	if err != nil {
		return err
	}
	ecs.labelFilter = labelFilter
	return nil
}

func (ecs *Ecs) createContainerStatusFilters() error {
	if len(ecs.ContainerStatusInclude) == 0 && len(ecs.ContainerStatusExclude) == 0 {
		ecs.ContainerStatusInclude = []string{"RUNNING"}
	}

	// ECS uses uppercase status names, normalizing for comparison.
	for i, include := range ecs.ContainerStatusInclude {
		ecs.ContainerStatusInclude[i] = strings.ToUpper(include)
	}
	for i, exclude := range ecs.ContainerStatusExclude {
		ecs.ContainerStatusExclude[i] = strings.ToUpper(exclude)
	}

	statusFilter, err := filter.NewIncludeExcludeFilter(ecs.ContainerStatusInclude, ecs.ContainerStatusExclude)
	if err != nil {
		return err
	}
	ecs.statusFilter = statusFilter
	return nil
}

func init() {
	inputs.Add("ecs", func() telegraf.Input {
		return &Ecs{
			EndpointURL:    "",
			Timeout:        config.Duration(5 * time.Second),
			newClient:      NewClient,
			filtersCreated: false,
		}
	})
}
