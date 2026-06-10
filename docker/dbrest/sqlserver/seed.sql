-- Seed data for the SQL Server client compatibility tests.
-- Uses MERGE for idempotency (upsert by primary key).
-- BIT columns use 0/1; JSON arrays stored as NVARCHAR(MAX).
-- IDENTITY columns require SET IDENTITY_INSERT ON for explicit ID inserts.

SET IDENTITY_INSERT dbo.todos ON;
MERGE dbo.todos AS tgt
USING (VALUES
    (1, 0, N'finish tutorial', CAST('2030-01-01' AS DATE), N'["go","sql"]'),
    (2, 1, N'pat the cat',     NULL,                       N'["pets"]'),
    (3, 0, N'do laundry',      CAST('2030-06-15' AS DATE), N'["chores","home"]')
) AS src (id, done, task, due, tags)
ON tgt.id = src.id
WHEN MATCHED THEN
    UPDATE SET done=src.done, task=src.task, due=src.due, tags=src.tags
WHEN NOT MATCHED THEN
    INSERT (id, done, task, due, tags) VALUES (src.id, src.done, src.task, src.due, src.tags);
SET IDENTITY_INSERT dbo.todos OFF;
GO

SET IDENTITY_INSERT dbo.persons ON;
MERGE dbo.persons AS tgt
USING (VALUES
    (1, N'Alice', 30, N'alice@example.com'),
    (2, N'Bob',   25, N'bob@example.com'),
    (3, N'Carol', 35, N'carol@example.com')
) AS src (id, name, age, email)
ON tgt.id = src.id
WHEN MATCHED THEN
    UPDATE SET name=src.name, age=src.age, email=src.email
WHEN NOT MATCHED THEN
    INSERT (id, name, age, email) VALUES (src.id, src.name, src.age, src.email);
SET IDENTITY_INSERT dbo.persons OFF;
GO

SET IDENTITY_INSERT dbo.assignments ON;
MERGE dbo.assignments AS tgt
USING (VALUES
    (1, 1, 1),
    (2, 2, 3)
) AS src (id, person_id, todo_id)
ON tgt.id = src.id
WHEN MATCHED THEN
    UPDATE SET person_id=src.person_id, todo_id=src.todo_id
WHEN NOT MATCHED THEN
    INSERT (id, person_id, todo_id) VALUES (src.id, src.person_id, src.todo_id);
SET IDENTITY_INSERT dbo.assignments OFF;
GO

SET IDENTITY_INSERT dbo.countries ON;
MERGE dbo.countries AS tgt
USING (VALUES
    (1, N'Germany'),
    (2, N'France'),
    (3, N'Japan')
) AS src (id, name)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET name=src.name
WHEN NOT MATCHED THEN INSERT (id, name) VALUES (src.id, src.name);
SET IDENTITY_INSERT dbo.countries OFF;
GO

SET IDENTITY_INSERT dbo.cities ON;
MERGE dbo.cities AS tgt
USING (VALUES
    (1, N'Berlin', 1),
    (2, N'Munich', 1),
    (3, N'Paris',  2),
    (4, N'Tokyo',  3)
) AS src (id, name, country_id)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET name=src.name, country_id=src.country_id
WHEN NOT MATCHED THEN INSERT (id, name, country_id) VALUES (src.id, src.name, src.country_id);
SET IDENTITY_INSERT dbo.cities OFF;
GO

SET IDENTITY_INSERT dbo.channels ON;
MERGE dbo.channels AS tgt
USING (VALUES
    (1, N'general'),
    (2, N'random'),
    (3, N'dev')
) AS src (id, slug)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET slug=src.slug
WHEN NOT MATCHED THEN INSERT (id, slug) VALUES (src.id, src.slug);
SET IDENTITY_INSERT dbo.channels OFF;
GO

SET IDENTITY_INSERT dbo.messages ON;
MERGE dbo.messages AS tgt
USING (VALUES
    (1, N'hello world',  1, 1),
    (2, N'how are you',  1, 2),
    (3, N'good morning', 2, 3),
    (4, N'deploy done',  3, 1)
) AS src (id, message, channel_id, person_id)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET message=src.message, channel_id=src.channel_id, person_id=src.person_id
WHEN NOT MATCHED THEN INSERT (id, message, channel_id, person_id) VALUES (src.id, src.message, src.channel_id, src.person_id);
SET IDENTITY_INSERT dbo.messages OFF;
GO

SET IDENTITY_INSERT dbo.companies ON;
MERGE dbo.companies AS tgt
USING (VALUES
    (1, N'Acme Corp', N'123 Main St', N'+1-555-0100'),
    (2, N'Globex',    N'456 Oak Ave', N'+1-555-0200')
) AS src (id, name, address, phone)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET name=src.name, address=src.address, phone=src.phone
WHEN NOT MATCHED THEN INSERT (id, name, address, phone) VALUES (src.id, src.name, src.address, src.phone);
SET IDENTITY_INSERT dbo.companies OFF;
GO

SET IDENTITY_INSERT dbo.people ON;
MERGE dbo.people AS tgt
USING (VALUES
    (1, N'John Doe',   30, 1),
    (2, N'Jane Smith', 25, 1),
    (3, N'Bob Jones',  40, 2)
) AS src (id, name, age, company_id)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET name=src.name, age=src.age, company_id=src.company_id
WHEN NOT MATCHED THEN INSERT (id, name, age, company_id) VALUES (src.id, src.name, src.age, src.company_id);
SET IDENTITY_INSERT dbo.people OFF;
GO

-- movies: UUID PK, seed with fixed UUIDs for reproducibility.
MERGE dbo.movies AS tgt
USING (VALUES
    (N'11111111-1111-1111-1111-111111111111', N'2001: A Space Odyssey', N'["sci-fi","classic"]',   CAST('1968-04-02' AS DATE), NULL),
    (N'22222222-2222-2222-2222-222222222222', N'The Matrix',            N'["sci-fi","action"]',    CAST('1999-03-31' AS DATE), NULL),
    (N'33333333-3333-3333-3333-333333333333', N'Mad Max: Fury Road',    N'["action","adventure"]', CAST('2015-05-15' AS DATE), NULL),
    (N'44444444-4444-4444-4444-444444444444', N'Interstellar',          N'["sci-fi","adventure"]', CAST('2014-11-05' AS DATE), NULL)
) AS src (id, name, tags, release_date, watched_at)
ON tgt.id = CAST(src.id AS UNIQUEIDENTIFIER)
WHEN MATCHED THEN UPDATE SET name=src.name, tags=src.tags, release_date=src.release_date, watched_at=src.watched_at
WHEN NOT MATCHED THEN INSERT (id, name, tags, release_date, watched_at)
    VALUES (CAST(src.id AS UNIQUEIDENTIFIER), src.name, src.tags, src.release_date, src.watched_at);
GO

SET IDENTITY_INSERT dbo.items ON;
MERGE dbo.items AS tgt
USING (VALUES
    (1, N'item one'),
    (2, N'item two')
) AS src (id, name)
ON tgt.id = src.id
WHEN MATCHED THEN UPDATE SET name=src.name
WHEN NOT MATCHED THEN INSERT (id, name) VALUES (src.id, src.name);
SET IDENTITY_INSERT dbo.items OFF;
GO
