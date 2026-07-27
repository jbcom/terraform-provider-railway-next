package changeset

func RegisterBucket(name, region string) Set {
	address := "bucket." + name
	resource := BucketResource{
		Address: address,
		Type:    "bucket",
		Name:    name,
		Config:  BucketConfig{Region: region},
	}
	return Set{
		Version: Version,
		Changes: []Change{{
			Kind:         "resource.create",
			Address:      address,
			Resource:     raw(resource),
			Path:         "resources." + address,
			Summary:      "Create bucket " + name,
			Severity:     "safe",
			DeployEffect: "none",
		}},
		Diagnostics: []Diagnostic{},
	}
}

func DeleteBucket(name, region string) Set {
	address := "bucket." + name
	resource := BucketResource{
		Address: address,
		Type:    "bucket",
		Name:    name,
		Config:  BucketConfig{Region: region},
	}
	return Set{
		Version: Version,
		Changes: []Change{{
			Kind:         "resource.delete",
			Address:      address,
			Previous:     raw(resource),
			Path:         "resources." + address,
			Summary:      "Delete bucket " + name,
			Severity:     "destructive",
			DeployEffect: "none",
		}},
		Diagnostics: []Diagnostic{},
	}
}
