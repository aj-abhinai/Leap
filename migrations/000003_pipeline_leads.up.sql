-- Pipelines, stages, programs, and leads. A lead has no identity columns of
-- its own: it MUST reference a contact (the single source of truth), so the
-- FK is NOT NULL. Contacts are soft-deleted in practice; the FK uses
-- RESTRICT so an accidental hard delete of a referenced contact fails loudly
-- instead of violating the NOT NULL constraint at runtime.

CREATE TABLE pipelines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- outcome is authoritative for what reaching a stage means for the lead:
-- open (in play), won, or lost. Closing stages must be won or lost — the
-- table constraint makes the column default 'open' invalid on a closing stage.
CREATE TABLE lead_stages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pipeline_id UUID NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    "order" INTEGER NOT NULL DEFAULT 0,
    color TEXT,
    is_closing BOOLEAN NOT NULL DEFAULT false,
    outcome TEXT NOT NULL DEFAULT 'open' CHECK (outcome IN ('open', 'won', 'lost')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lead_stages_closing_outcome_check CHECK (NOT is_closing OR outcome IN ('won', 'lost'))
);

-- Fixed-price program catalog. Lead values are price snapshots taken at lead
-- creation; catalog price changes never rewrite existing leads, so deleting a
-- referenced program is refused (RESTRICT).
CREATE TABLE programs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    price DECIMAL(12,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE leads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contact_id UUID NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    pipeline_id UUID NOT NULL REFERENCES pipelines(id) ON DELETE RESTRICT,
    stage_id UUID NOT NULL REFERENCES lead_stages(id) ON DELETE RESTRICT,
    value DECIMAL(12,2),
    notes TEXT,
    program_id UUID REFERENCES programs(id) ON DELETE RESTRICT,
    nickname TEXT,
    -- outcome/lost_reason are denormalized snapshots resolved from the stage's
    -- outcome metadata when the lead moves; lost_reason is user-supplied.
    outcome TEXT CHECK (outcome IN ('won','lost')),
    lost_reason TEXT,
    assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_leads_program_id ON leads (program_id);
CREATE INDEX idx_leads_pipeline_id_active ON leads (pipeline_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_leads_stage_id_active ON leads (stage_id) WHERE deleted_at IS NULL;

-- Stage-move history with the stage names captured at move time, so renaming
-- or deleting a stage later does not rewrite history.
CREATE TABLE lead_stage_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lead_id UUID NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
    from_stage_id UUID REFERENCES lead_stages(id) ON DELETE SET NULL,
    to_stage_id UUID REFERENCES lead_stages(id) ON DELETE SET NULL,
    from_stage_name TEXT NOT NULL DEFAULT '',
    to_stage_name TEXT NOT NULL DEFAULT '',
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    moved_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_lead_stage_history_lead ON lead_stage_history(lead_id);
