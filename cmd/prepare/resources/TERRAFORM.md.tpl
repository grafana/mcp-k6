# Terraform resources for k6 Cloud

The Grafana Cloud Terraform provider (https://registry.terraform.io/providers/grafana/grafana/latest/docs) includes resources for k6 Cloud, since k6 Cloud is a part of Grafana Cloud.
Using this provider, it is possible to manage many different k6 Cloud-related resources, as well as querying data from them (using their counterpart data sources).

## Basics

The configuration of the Grafana Terraform provider itself is outside the scope of this guide. The agent should assume that the user has already configured it. Additionally, a Grafana Cloud stack is required to use k6 Cloud. The creation of a Grafana Stack is also outside the scope of this guide, and the agent should assume that the user has already created it.

The core k6 Cloud entities are:
- Projects: They belong to a Grafana Stack.
- Load Tests: Load tests belong to a project. They define a k6 script that can be run in the future.
- Load Test Runs: These represent runs of a load test. They are not represented in Terraform in any way.
- Schedules: Schedules are assigned to load tests. They define regular schedules for running load tests (for example, once a day).

Note that the `grafana_k6_installation` resource can be mostly ignored, unless the user explicitly asks for a k6 Cloud installation resource.

## Resources
{{range .Resources}}
### {{.Name}}

{{.Description}}

Attributes:

```json
{{.JSON}}
```
{{end}}

## Data Sources
{{range .DataSources}}
### {{.Name}}

{{.Description}}

Attributes:

```json
{{.JSON}}
```
{{end}}
