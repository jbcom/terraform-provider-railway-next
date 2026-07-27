package changeset

type DatabaseResource struct {
	Address          string          `json:"address"`
	Type             string          `json:"type"`
	Kind             string          `json:"kind"`
	Engine           string          `json:"engine"`
	Name             string          `json:"name"`
	Image            string          `json:"image"`
	Output           string          `json:"output"`
	DefaultMountPath string          `json:"defaultMountPath"`
	Source           DatabaseSource  `json:"source"`
	Deploy           *DatabaseDeploy `json:"deploy,omitempty"`
}

type DatabaseSource struct {
	Type  string `json:"type"`
	Image string `json:"image"`
}

type DatabaseDeploy struct {
	MultiRegionConfig map[string]map[string]int `json:"multiRegionConfig"`
}

func CreatePostgres(name, version, region string) Set {
	resource := postgresResource(name, version, region)
	return Set{
		Version: Version,
		Changes: []Change{{
			Kind:         "resource.create",
			Address:      resource.Address,
			Resource:     raw(resource),
			Path:         "resources." + resource.Address,
			Summary:      "Create database " + name,
			Severity:     "safe",
			DeployEffect: "deploy",
		}},
		Diagnostics: []Diagnostic{},
	}
}

func DeletePostgres(name, version, region string) Set {
	resource := postgresResource(name, version, region)
	return Set{
		Version: Version,
		Changes: []Change{{
			Kind:         "resource.delete",
			Address:      resource.Address,
			Previous:     raw(resource),
			Path:         "resources." + resource.Address,
			Summary:      "Delete database " + name,
			Severity:     "destructive",
			DeployEffect: "deploy",
		}},
		Diagnostics: []Diagnostic{},
	}
}

func postgresResource(name, version, region string) DatabaseResource {
	image := "ghcr.io/railwayapp-templates/postgres-ssl:" + version
	resource := DatabaseResource{
		Address:          "database." + name,
		Type:             "database",
		Kind:             "database",
		Engine:           "postgres",
		Name:             name,
		Image:            image,
		Output:           "DATABASE_URL",
		DefaultMountPath: "/var/lib/postgresql/data",
		Source: DatabaseSource{
			Type:  "image",
			Image: image,
		},
	}
	if region != "" {
		resource.Deploy = &DatabaseDeploy{
			MultiRegionConfig: map[string]map[string]int{
				region: {"numReplicas": 1},
			},
		}
	}
	return resource
}
