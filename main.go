package main

import (
	"os"
	"log"
	"gopkg.in/yaml.v2"
	"fmt"
	"github.com/concourse/atc"
	"github.com/knative/build/pkg/apis/build/v1alpha1"
)

func main() {
	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("could not open %s: %s", os.Args[1], err.Error())
	}

	var build v1alpha1.Build
	err = yaml.NewDecoder(file).Decode(&build)
	if err != nil {
		log.Fatalf("could not parse %s: %s", os.Args[1], err.Error())
	}

	var pipeline struct {
		Resources []atc.ResourceConfig `yaml:"resources"`
		Jobs      []atc.JobConfig      `yaml:"jobs"`
	}

	if build.Spec.Source.Git == nil {
		log.Fatalf("no 'git' source type was provided")
	} else if build.Spec.Source.GCS != nil {
		log.Fatalf("can't convert 'gcs' source type")
	} else if build.Spec.Source.Custom != nil {
		log.Fatalf("can't convert 'custom' source type")
	}

	gitResource := atc.ResourceConfig{
		Name: "git-resource-from-knative-build",
		Type: "git",
		Source: atc.Source{
			"uri":    build.Spec.Source.Git.Url,
			"branch": build.Spec.Source.Git.Revision,
		},
	}
	pipeline.Resources = append(pipeline.Resources, gitResource)

	job := atc.JobConfig{
		Name: fmt.Sprintf("job-%s-from-knative-build", build.GetName()),
		Plan: atc.PlanSequence{},
	}
	for _, s := range build.Spec.Steps {
		planStep := atc.PlanConfig{
			Task: s.Name,
			TaskConfig: &atc.TaskConfig{
				Platform: "linux",
				ImageResource: &atc.ImageResource{
					Type: "docker-image",
					Source: atc.Source{
						"repository": s.Image,
					},
				},
			},
		}

		if len(s.Command) > 0 {
			planStep.TaskConfig.Run = atc.TaskRunConfig{
				Path: s.Command[0],
				Args: append(s.Command[0:], s.Args...),
			}
		} else {
			log.Println("cannot currently read ENTRYPOINTs, so you will need to work it out and manually edit your pipeline")
			planStep.TaskConfig.Run = atc.TaskRunConfig{
				Path: "set this to the correct path from ENTRYPOINT",
				Args: append(s.Command[0:], s.Args...),
			}
		}

		for _, p := range s.Env {
			planStep.Params[s.Name] = p.Value
		}

		for _, i := range s.VolumeMounts {
			planStep.TaskConfig.Inputs = append(planStep.TaskConfig.Inputs, atc.TaskInputConfig{
				Name: i.Name,
				Path: i.SubPath, // TODO: is this right??
			})
		}

		job.Plan = append(job.Plan, planStep)
	}

	pipeline.Jobs = append(pipeline.Jobs, job)

	yaml.NewEncoder(os.Stdout).Encode(pipeline)
}
