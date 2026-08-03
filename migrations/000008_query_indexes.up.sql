CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX idx_leads_pipeline_id_active ON leads (pipeline_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_leads_stage_id_active ON leads (stage_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_user_id ON audit_logs (user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs (action);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs (resource_type);
