-- Lead activities and the audit log.

-- An activity is a task on a lead: calls, follow-ups, notes. It carries a
-- "happened at" time (occurred_at when logged as done, responded_at for the
-- quick-reply response time), a cancellable state, and optional scheduling
-- (scheduled_at) and reminder (remind_at) times. quick_reply_id links the
-- activity to the tags row (type='quick_reply') recording what happened.
CREATE TABLE lead_activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    stage_id UUID NOT NULL REFERENCES lead_stages(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type TEXT NOT NULL DEFAULT 'note',
    description TEXT NOT NULL DEFAULT '',
    scheduled_at TIMESTAMPTZ,
    -- scheduled_end_at is the optional end of a range task ("3:00–5:00pm").
    -- The nudge fires before the start; overdue begins after the end.
    scheduled_end_at TIMESTAMPTZ,
    remind_at TIMESTAMPTZ,
    occurred_at TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,
    quick_reply_id UUID REFERENCES tags(id) ON DELETE SET NULL,
    is_done BOOLEAN NOT NULL DEFAULT false,
    is_cancelled BOOLEAN NOT NULL DEFAULT false,
    is_reminded BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_lead_activities_lead ON lead_activities(lead_id);
-- Undelivered reminders: the reminder scan polls rows with a due remind_at
-- that have not been marked reminded yet.
CREATE INDEX idx_lead_activities_remind ON lead_activities(remind_at) WHERE remind_at IS NOT NULL AND NOT is_reminded;
-- Next-task preview on lead cards: open, scheduled, non-cancelled tasks.
CREATE INDEX idx_lead_activities_next_task
    ON lead_activities(lead_id, is_done, scheduled_at)
    WHERE scheduled_at IS NOT NULL AND NOT is_done AND NOT is_cancelled;

-- Every mutation is recorded with who did what and when, including a JSON
-- diff of the changes. The activity feed orders by created_at DESC and
-- filters by user, so only those are indexed; action/resource_type filters
-- are low-cardinality and served fine by the created_at-ordered scan.
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    description TEXT NOT NULL,
    user_id UUID,
    user_name TEXT,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id UUID,
    changes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id);

-- Org-wide settings (key-value). The nudge lead time ("how many minutes
-- before the start time the reminder fires") is the first setting; defaults
-- to 5 when absent (ADR 004).
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
