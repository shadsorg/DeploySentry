ALTER TABLE deployments
    ADD CONSTRAINT fk_deployments_release_id
    FOREIGN KEY (release_id) REFERENCES releases(id) ON DELETE CASCADE;
