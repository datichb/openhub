package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// N4 (v5 critical, DC4): the operator reads freely, asks for secrets,
// never changes anything while `apply` is locked, and never prints
// credentials; global options before the sub-command do not help.
func TestInfraReadOnly(t *testing.T) {
	rules := effectiveRules(t, "operator", "operator")
	for _, cmd := range []string{
		"terraform plan", "terraform -chdir=infra validate", "terraform state list", "kubectl get pods -A",
		"kubectl describe deploy api", "kubectl logs api-1", "kubectl diff -f k8s/", "helm template ./chart", "helm list -A",
		"helm diff upgrade api ./chart -f values.yaml", "helm diff upgrade --install api ./chart",
		"aws sts get-caller-identity", "aws ec2 describe-instances", "aws s3 ls", "gcloud compute instances list",
		"az group list", "pulumi preview",
	} {
		eff := ShellEffect(rules, cmd)
		if cmd == "terraform -chdir=infra validate" {
			assert.Equal(t, sessionspec.EffectAsk, eff, "a global option first: the agent's own rule (ask)")
			continue
		}
		assert.Equal(t, sessionspec.EffectAllow, eff, cmd)
	}
	for _, cmd := range []string{
		"kubectl get secret db -o yaml", "terraform output", "helm get values api",
		"aws ssm get-parameter --name x", "kubectl port-forward svc/db 5432",
	} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	mutations := []string{
		"terraform apply", "terraform -chdir=x apply -auto-approve", "terraform destroy", "tofu apply",
		"kubectl apply -f k8s/", "kubectl --context prod delete pod x", "kubectl exec -it api -- sh", "kubectl apply --dry-run=none -f x",
		"helm upgrade api ./chart", "helm uninstall api", "helm --kube-context prod upgrade --install api ./chart",
		"/usr/local/bin/helm rollback api 3", "aws --profile p ec2 terminate-instances --instance-ids i-1",
		"aws s3 rm s3://b/x", "aws lambda invoke --function-name f out", "gcloud compute instances delete x", "az group delete -n x",
		"pulumi up", "vault kv put secret/x a=b", "vault login",
	}
	for _, cmd := range mutations {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), "asked to the agent: %s", cmd)
	}
	for _, cmd := range []string{
		"aws secretsmanager get-secret-value --secret-id x", "aws ssm get-parameter --name x --with-decryption",
		"aws ecr get-login-password", "aws sts assume-role --role-arn x", "gcloud auth print-access-token",
		"pulumi stack output --show-secrets", "kubectl config view --raw", "VAULT_TOKEN=x vault read x",
	} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}

	// Session rules: refused while `apply` is locked, and in a workflow
	// that declares no checkpoint unlocking it; asked once it is unlocked.
	none := &sessionspec.WorkflowRuntime{ID: "libre", DefaultMode: "manuel"}
	gated := &sessionspec.WorkflowRuntime{ID: "infra-plan", DefaultMode: "manuel", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-apply", Behaviors: map[string]string{"manuel": "pause"}, Unlocks: []string{sessionspec.UnlockApply}},
	}}
	unlocked := &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockApply: {Checkpoint: "cp-apply"}}}
	for _, cmd := range mutations {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(append(rules, SessionRules(none, "manuel", nil)...), cmd), "no apply checkpoint: %s", cmd)
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(append(rules, SessionRules(gated, "manuel", nil)...), cmd), "locked: %s", cmd)
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(append(rules, SessionRules(gated, "manuel", unlocked)...), cmd), "unlocked: %s", cmd)
	}
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(append(rules, SessionRules(none, "manuel", nil)...), "kubectl get pods"))

	// The families the operator does not run (vault, ansible): guarded the
	// same way for an agent that runs them as a whole.
	other := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "deny", "vault *": "ask", "ansible*": "ask"}})
	other = append(other, InfraGuard(other)...)
	for _, cmd := range []string{"vault status", "ansible-lint site.yml", "ansible-playbook site.yml --syntax-check"} {
		assert.Equal(t, sessionspec.EffectAllow, ShellEffect(other, cmd), cmd)
	}
	for _, cmd := range []string{"vault kv get secret/db", "ansible-playbook site.yml --check", "vault kv put secret/x a=b"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(other, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(other, "kubectl get pods"), "a family it does not run")

	// The guard never widens an agent that does not run the tools.
	dev := effectiveRules(t, "developer", "developer")
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(dev, "kubectl get pods"))
	assert.Empty(t, InfraGuard(ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "deny", "git *": "allow"}})))
}

// C1: the auditor-subagent names a few analysis sub-commands (infra audit
// domain): they stay asked (the guard, for agents that run a tool as a
// whole, does not apply), everything else is refused, changes included.
func TestAuditorSubagentInfraTools(t *testing.T) {
	rules := effectiveRules(t, "auditor", "auditor-subagent")
	for _, cmd := range []string{
		"terraform validate", "terraform plan -input=false -detailed-exitcode", "terraform init -backend=false -lockfile=readonly",
		"terraform fmt -check -recursive", "tflint --recursive", "tfsec .", "checkov -d . --compact", "trivy config .",
		"kube-score score k8s/deploy.yaml", "kubesec scan k8s/deploy.yaml", "helm lint ./chart", "helm template ./chart",
		"infracost breakdown --path infra",
	} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{
		"terraform apply", "terraform init", "terraform fmt", "terraform output", "terraform state pull", "kubectl get pods",
		"kubectl apply -f k8s/", "helm upgrade api ./chart", "aws s3 ls", "trivy image alpine", "cat .env", "rm -rf infra",
	} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
	assert.Empty(t, InfraGuard(rules), "no infra guard for named sub-commands")
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, "infra/main.tf"))
}

// N4 (DC3): the debugger reads and diagnoses; it asks before running the
// project's tests and cannot change files or run anything else.
func TestDebuggerShell(t *testing.T) {
	rules := effectiveRules(t, "debugger", "debugger")
	for _, cmd := range []string{"git log -5 --oneline", "git blame src/a.go", "rg 'panic' src", "tail -100 app.log", "bd show pt-1", "bd create 'Fix x'"} {
		assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{"go test ./...", "npm test", "pytest -x tests/", ".venv/bin/python -m pytest"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{"sed -i s/a/b/ x.go", "python3 -c 'print(1)'", "node -e 1", "curl https://x", "make", "rm x"} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, "src/a.go"))
}

// N4: the benchmarker no longer reads the environment and asks before the
// tools that run an arbitrary command.
func TestBenchmarkerShell(t *testing.T) {
	rules := effectiveRules(t, "benchmarker", "benchmarker")
	for _, cmd := range []string{"env", "env FOO=1 sh -c x", "printenv AWS_SECRET_ACCESS_KEY"} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
	for _, cmd := range []string{"hyperfine 'sh -c x'", "perf record -- ./x", "wrk -s x.lua http://localhost"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, "go test -bench=. ./..."))
	// C0.9: load tools and curl hit a target to confirm; results only go to
	// reports/benchmarks/ (the benchmarker never modifies the source).
	for _, cmd := range []string{"k6 run load.js", "ab -n 1000 http://localhost/", "hey -z 10s http://x", "curl -s https://api.example.com"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rules, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, "mkdir -p reports/benchmarks"))
	assert.Equal(t, sessionspec.EffectAllow, ToolEffect(rules, sessionspec.ActionEdit, "reports/benchmarks/2026-10-12-api.md"))
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, "src/main.go"))
}
