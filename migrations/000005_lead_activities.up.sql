CREATE TABLE lead_activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    stage_id UUID NOT NULL REFERENCES lead_stages(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type TEXT NOT NULL DEFAULT 'note',
    description TEXT NOT NULL DEFAULT '',
    scheduled_at TIMESTAMPTZ,
    remind_at TIMESTAMPTZ,
    is_done BOOLEAN NOT NULL DEFAULT false,
    is_reminded BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_lead_activities_lead ON lead_activities(lead_id);
CREATE INDEX idx_lead_activities_remind ON lead_activities(remind_at) WHERE remind_at IS NOT NULL AND NOT is_reminded;
