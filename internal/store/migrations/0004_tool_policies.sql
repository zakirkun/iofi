-- Grant the tools added alongside internal/toolcat.
--
-- Three role prompts in 0002 instructed agents to do things no tool let them
-- do: tech-leader was told to "set depends_on", sprint-manager was told it
-- "may reassign", and finance was told to read run cost while its policy
-- denied cost_report. Those are fixed here, not by softening the prompts.
--
-- Each allow list is replaced wholesale rather than appended to, so the policy
-- in the database matches the one written down here instead of depending on
-- what a previous migration happened to leave behind. Roles the founder edited
-- through the UI are overwritten — that is the cost of a seeded default, and
-- it is why custom roles should be created rather than seeded ones renamed.

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_split","task_update",
  "task_assign","task_depend","task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search","memory_update","memory_forget",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_open","meeting_post","meeting_read","meeting_list","meeting_close",
  "handoff","ask_role","escalate_founder",
  "repo_status","repo_diff","repo_log",
  "cost_report","run_list","run_tail"]}'::jsonb
WHERE slug = 'cto';

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_split","task_update",
  "task_assign","task_depend","task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search","memory_update",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_open","meeting_post","meeting_read","meeting_list","meeting_close",
  "handoff","ask_role","escalate_founder",
  "repo_status","repo_diff","repo_log","pr_open","pr_status",
  "run_list","run_tail"]}'::jsonb
WHERE slug = 'tech-leader';

-- task_get was missing: a programmer could claim a task but not read its spec
-- outside the brief it was dispatched with.
UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_claim","task_update","task_comment",
  "task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","skill_list","skill_get",
  "meeting_post","meeting_read","meeting_list","ask_role","escalate_founder",
  "repo_status","repo_diff","repo_log","pr_open","pr_status"]}'::jsonb
WHERE slug = 'lead-programmer';

-- QA verifies work it did not write, which is impossible without repo_diff.
UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_claim","task_create","task_update",
  "task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_post","meeting_read","meeting_list","handoff","ask_role","escalate_founder",
  "repo_status","repo_diff","repo_log","pr_status"]}'::jsonb
WHERE slug = 'qa';

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_claim","task_create","task_update",
  "task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","skill_list","skill_get",
  "meeting_post","meeting_read","meeting_list","ask_role","escalate_founder",
  "repo_status","repo_diff","repo_log","pr_open","pr_status",
  "run_list","run_tail"]}'::jsonb
WHERE slug = 'devops';

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_split","task_update",
  "task_depend","task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","skill_list","skill_get",
  "meeting_open","meeting_post","meeting_read","meeting_list","meeting_close",
  "ask_role","escalate_founder"]}'::jsonb
WHERE slug = 'business-analyst';

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_update","task_depend",
  "task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_open","meeting_post","meeting_read","meeting_list","meeting_close",
  "ask_role","escalate_founder",
  "run_list"]}'::jsonb
WHERE slug = 'business-process';

-- Finance was told to "read cost from run data before claiming a number" while
-- its policy denied every tool that reports one.
UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_update","task_comment",
  "memory_write","memory_search",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_post","meeting_read","meeting_list","ask_role","escalate_founder",
  "cost_report","run_list","run_tail"]}'::jsonb
WHERE slug = 'finance';

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_split","task_update",
  "task_assign","task_depend","task_comment","task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_open","meeting_post","meeting_read","meeting_list","meeting_close",
  "handoff","ask_role","escalate_founder",
  "cost_report","run_list","run_tail"]}'::jsonb
WHERE slug = 'sprint-manager';

UPDATE roles SET tool_policy = '{"allow":[
  "task_list","task_get","task_search","task_create","task_update","task_comment",
  "task_label","kanban_move","board_get",
  "memory_write","memory_search",
  "role_list","agent_roster","skill_list","skill_get",
  "meeting_post","meeting_read","meeting_list","handoff","ask_role","escalate_founder"]}'::jsonb
WHERE slug = 'support-ticket';

-- The prompts promised capabilities the policies withheld. Say so where the
-- agent actually reads it.
UPDATE roles SET system_prompt = system_prompt ||
  E'\nCall role_list before any handoff or ask_role: a guessed slug fails.\n' ||
  E'Call agent_roster before assigning work to a person rather than a role.'
WHERE slug IN ('cto','tech-leader','sprint-manager','qa','business-process','support-ticket');
