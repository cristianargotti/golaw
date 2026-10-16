[.workflow_runs[] |
  select(.head_sha == $sha and
    .head_branch == "main" and .event == "push")] |
sort_by(.run_number, .run_attempt) | last |
.status == "completed" and .conclusion == "success"
