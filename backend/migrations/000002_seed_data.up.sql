-- 2. Сидирование 6 демо-пользователей
INSERT INTO users (id, login, password_hash, role, inn, display_name)
VALUES
    ('00000000-0000-0000-0000-000000000001', 'client_alfa', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'CLIENT', '7701000001', 'ООО «Альфа Логистика»'),
    ('00000000-0000-0000-0000-000000000002', 'client_smirnov', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'CLIENT', '7701000002', 'ИП Смирнов Алексей Петрович'),
    ('00000000-0000-0000-0000-000000000003', 'client_technostart', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'CLIENT', '7701000003', 'ООО «ТехноСтарт»'),
    ('00000000-0000-0000-0000-000000000004', 'client_negativ', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'CLIENT', '7701000004', 'ООО «Негатив Трейд»'),
    ('00000000-0000-0000-0000-000000000005', 'client_megastroy', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'CLIENT', '7701000005', 'ООО «Мегастрой»'),
    ('00000000-0000-0000-0000-000000000099', 'underwriter', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'UNDERWRITER', NULL, 'Главный Андеррайтер')
ON CONFLICT (login) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    role = EXCLUDED.role,
    inn = EXCLUDED.inn,
    display_name = EXCLUDED.display_name;
