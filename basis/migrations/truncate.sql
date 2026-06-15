-- Development cleanup: wipe all data and reset sequences.
-- DO NOT run in production.
TRUNCATE memories, messages, conversations, prosopons RESTART IDENTITY CASCADE;

-- Re-seed the internal Celine system prosopon.
INSERT INTO prosopons (id, sub, email, display_name)
VALUES (1, 'celine', 'celine@internal', 'Celine');

SELECT setval('prosopons_id_seq', 1);
