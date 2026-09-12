-- Lookup and foreign-key indexes missing from the earlier schema. Plain
-- CREATE INDEX (not CONCURRENTLY): golang-migrate wraps each migration in a
-- transaction, and these tables are small at this stage.

-- Tag delete guards and settings usage counts scan the join table by tag.
CREATE INDEX idx_contact_tags_tag_id ON contact_tags(tag_id);
-- Status delete guards count contacts by status_id.
CREATE INDEX idx_contacts_status_id ON contacts(status_id);
-- Stage loads and delete guards filter by pipeline.
CREATE INDEX idx_lead_stages_pipeline_id ON lead_stages(pipeline_id);
-- The board and reminder feeds filter leads by assignee.
CREATE INDEX idx_leads_assigned_to ON leads(assigned_to) WHERE deleted_at IS NULL;
-- Quick-reply delete guards count activities; the reminders feed filters by them.
CREATE INDEX idx_lead_activities_quick_reply_id ON lead_activities(quick_reply_id);
-- Stage delete guards count activities that still link a stage.
CREATE INDEX idx_lead_activities_stage_id ON lead_activities(stage_id);

-- Phone duplicate/resolve lookups match both the stored digits and the legacy
-- national form (leading zeros stripped). The digits expression index existed;
-- without a matching index the second OR arm forced a scan.
CREATE INDEX idx_contact_phones_national ON contact_phones (ltrim(regexp_replace(value, '\D', '', 'g'), '0'));
