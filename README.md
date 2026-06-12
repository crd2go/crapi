# CRAPI

`CRAPI` (CR <-> API) is a Go library for Kubernetes operators that need to bridge the gap between a Custom Resource (CR) and an external API.

When an operator manages resources backed by an HTTP API, it must translate the Kubernetes-shaped spec into an API request, and then map the API response back into a CR status (or a full spec+status round-trip). `crapi` makes both directions easy, including edge cases like resolving Kubernetes object references (e.g. `GroupRef`) and fetching sensitive values from Kubernetes `Secret` objects before forwarding them to the API.

## How it works

The `Translator` interface is the core abstraction:

```go
type Translator interface {
    // ToAPI converts a CR spec into an API request struct.
    // Dependency objects (Secrets, referenced CRs) are passed as extra arguments
    // so the translator can resolve references before producing the request.
    ToAPI(target any, source client.Object, objs ...client.Object) error

    // FromAPI converts an API response into a CR, populating both spec and status.
    // Any values extracted into separate Kubernetes objects (e.g. Secrets) are
    // returned as extra objects.
    FromAPI(target client.Object, source any, objs ...client.Object) ([]client.Object, error)
}
```

Create a `Translator` with `NewTranslator` (single API version) or `NewPerVersionTranslators` (multiple versions indexed by SDK major version).

## Example

The examples below are derived from the test suite and use a `Group` (Atlas project) resource.

### API response → CR (`FromAPI`)

```go
// API response from the Atlas API
apiGroup := admin2025.Group{
    Id:           ptr("6127378123219"),
    Name:         "test-project",
    OrgId:        "60987654321654321",
    ClusterCount: 0,
    Created:      time.Date(2025, 1, 1, 1, 30, 15, 0, time.UTC),
    Tags: &[]admin2025.ResourceTag{
        {Key: "env", Value: "prod"},
    },
    WithDefaultAlertsSettings: ptr(true),
}

// Target CR to be populated
cr := &samplesv1.Group{
    Spec: samplesv1.GroupSpec{
        V20250312: &samplesv1.GroupSpecV20250312{
            ProjectOwnerId: ptr(""),  // read-only field preserved from spec
        },
    },
}

_, err := tr.FromAPI(cr, &apiGroup)
// cr.Spec.V20250312.Entry  → writable fields: Name, OrgId, Tags, ...
// cr.Status.V20250312      → read-only fields: Id, Created, ClusterCount, ...
```

### CR spec → API request (`ToAPI`)

```go
// CR with the desired state
cr := &samplesv1.Group{
    Spec: samplesv1.GroupSpec{
        V20250312: &samplesv1.GroupSpecV20250312{
            Entry: &samplesv1.GroupSpecV20250312Entry{
                Name:                      "project-name",
                OrgId:                     "60987654321654321",
                WithDefaultAlertsSettings: ptr(true),
                Tags: &[]samplesv1.Tags{
                    {Key: "env", Value: "prod"},
                },
            },
        },
    },
}

// Empty target struct to be filled
var req admin2025.Group

err := tr.ToAPI(&req, cr)
// req is now ready to pass directly to the Atlas SDK client
```

### Resolving references and secrets (`ToAPI` with dependencies)

When a CR refers to another CR by name (e.g. `GroupRef`) or stores sensitive values in a `Secret`, pass those objects as dependencies. The translator resolves the references automatically:

```go
cr := &samplesv1.GroupAlertsConfig{
    Spec: samplesv1.GroupAlertsConfigSpec{
        V20250312: &samplesv1.GroupAlertsConfigSpecV20250312{
            GroupRef: &crd2gok8s.LocalReference{Name: "my-project"},
            Entry: &samplesv1.GroupAlertsConfigSpecV20250312Entry{
                Notifications: &[]samplesv1.Notifications{
                    {
                        DatadogApiKeySecretRef: &samplesv1.PasswordSecretRef{
                            Name: "datadog-secret",
                        },
                        DatadogRegion: ptr("US"),
                    },
                },
            },
        },
    },
}

deps := []client.Object{
    // Referenced Group CR — its status.Id is injected as GroupId in the request
    &samplesv1.Group{
        ObjectMeta: metav1.ObjectMeta{Name: "my-project"},
        Status: samplesv1.GroupStatus{
            V20250312: &samplesv1.GroupStatusV20250312{Id: ptr("62b6e34b3d91647abb20e7b8")},
        },
    },
    // Secret referenced in the spec
    &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{Name: "datadog-secret", Namespace: "ns"},
        Data:       map[string][]byte{"password": []byte("dd-api-key-value")},
    },
}

var req admin2025.GroupAlertsConfig
err := tr.ToAPI(&req, cr, deps...)
// req.GroupId           == "62b6e34b3d91647abb20e7b8"  (resolved from GroupRef)
// req.Notifications[0].DatadogApiKey == "dd-api-key-value"  (resolved from Secret)
```

## License

Apache License 2.0
