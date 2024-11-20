CREATE TABLE
    `branches` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `name` varchar(4096) NOT NULL DEFAULT '' comment '分支名',
        `rid` bigint (20) unsigned NOT NULL comment '存储库 ID',
        `hash` char(64) NOT NULL DEFAULT '' comment '分支提交',
        `protection_level` int (11) NOT NULL DEFAULT '0' comment '保护分支级别，普通 0，保护分支 10，归档 20，隐藏分支 30',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_branches_rid_name` (`rid`, `name`) LOCAL,
        KEY `idx_branches_rid` (`rid`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '分支表';

CREATE TABLE
    `objects` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `rid` bigint (20) unsigned NOT NULL comment '仓库 ID',
        `hash` char(64) NOT NULL DEFAULT '' comment '对象哈希值',
        `bindata` mediumblob NOT NULL comment '编码对象',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_objects_rid_hash` (`rid`, `hash`) LOCAL,
        KEY `idx_objects_rid` (`rid`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '扩展元数据对象表';

CREATE TABLE
    `commits` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `rid` bigint (20) unsigned NOT NULL comment '仓库 ID',
        `hash` char(64) NOT NULL DEFAULT '' comment '提交哈希值',
        `author` varchar(512) NOT NULL DEFAULT '' comment '作者邮箱',
        `committer` varchar(512) NOT NULL DEFAULT '' comment '提交者邮箱',
        `bindata` mediumblob NOT NULL comment '编码对象',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间，以 author when 填充',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '修改时间，以 committer when 填充',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_commits_rid_hash` (`rid`, `hash`) LOCAL,
        KEY `idx_commits_rid` (`rid`) LOCAL,
        KEY `idx_commits_author` (`author`) LOCAL,
        KEY `idx_commits_committer` (`committer`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '提交表';

CREATE TABLE
    `trees` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `rid` bigint (20) unsigned NOT NULL comment '存储库 ID',
        `hash` char(64) NOT NULL comment 'tree 哈希值 - 16 进制',
        `bindata` mediumblob NOT NULL comment '编码对象',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_trees_rid_hash` (`rid`, `hash`) LOCAL,
        KEY `idx_trees_rid` (`rid`) LOCAL
    ) AUTO_INCREMENT = 1 DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = 'tree 表';

CREATE TABLE
    `tags` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `rid` bigint (20) unsigned NOT NULL comment '存储库 ID',
        `uid` bigint (20) unsigned NOT NULL DEFAULT '0' comment '创建者的 ID',
        `name` varchar(4096) NOT NULL comment '标签名',
        `hash` char(64) NOT NULL comment 'Tag 哈希值',
        `subject` varchar(1024) NOT NULL DEFAULT 'CURRENT_TIMESTAMP' comment 'Tag 标题',
        `description` mediumtext NOT NULL comment 'Tag 描述信息',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_tags_rid_name` (`rid`, `name`) LOCAL,
        KEY `idx_tags_rid` (`rid`) LOCAL
    ) AUTO_INCREMENT = 1 DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '引用表';

CREATE TABLE
    `namespaces` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `name` varchar(256) NOT NULL comment 'namespace 展示名',
        `path` varchar(256) NOT NULL comment 'namespace 路径',
        `description` varchar(512) NOT NULL comment 'namespace 描述信息',
        `type` tinyint (4) NOT NULL DEFAULT '0' comment 'namespace 类型，0 UserNamespace，1 GroupNamespace。',
        `owner_id` bigint (20) unsigned NOT NULL comment '所有者 ID',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_namespaces_path` (`path`) GLOBAL,
        KEY `idx_namespaces_type_owner_id` (`type`, `owner_id`) GLOBAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = 'namespaces';

CREATE TABLE
    `users` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `username` char(255) NOT NULL comment '域账号/用户名',
        `name` varchar(255) NOT NULL comment '昵称',
        `admin` tinyint (4) NOT NULL comment '是否为管理员',
        `email` varchar(255) NOT NULL comment '邮箱',
        `type` tinyint (4) NOT NULL DEFAULT '0' COMMENT '用户类型，0 普通用户，1 bot, 2 外包',
        `password` varchar(512) NOT NULL DEFAULT '' comment '加盐后哈希的密码，校验时使用特定的算法校验，eg argon2:encrypt123456.',
        `signature_token` varchar(255) NOT NULL DEFAULT '' comment '随机生成的签名 Token，用于安全签名',
        `locked_at` timestamp NULL DEFAULT NULL COMMENT '锁定时间，NULL 未锁定',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_users_username` (`username`) LOCAL,
        KEY `idx_users_on_name` (`name`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '用户表';

CREATE TABLE
    `repositories` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `name` varchar(256) NOT NULL comment '存储库名称',
        `path` varchar(256) NOT NULL comment '存储库路径',
        `namespace_id` bigint (20) unsigned NOT NULL comment '所属 namespace',
        `description` text NOT NULL comment '存储库描述信息',
        `default_branch` varchar(4096) NOT NULL comment '默认分支名',
        `hash_algo` char(64) NOT NULL DEFAULT 'BLAKE3' comment '哈希算法',
        `compression_algo` char(64) NOT NULL DEFAULT 'zstd' comment '压缩算法',
        `visible_level` int (11) NOT NULL DEFAULT '0' comment '0 私有，10 内部员工可读，20 外包可读，30 匿名可读',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        `deleted_at` bigint (20) unsigned NOT NULL DEFAULT '0' comment '存储库标记删除时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_repositories_namespace_path` (`namespace_id`, `path`) GLOBAL,
        KEY `idx_repositories_namespace` (`namespace_id`) GLOBAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '存储库表';

CREATE TABLE
    `members` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `rid` bigint (20) unsigned NOT NULL comment '资源 ID（存储库 ID 或群组 ID）', -- Note the difference from the repository ID
        `uid` bigint (20) unsigned NOT NULL comment '用户 ID',
        `access_level` int (10) unsigned NOT NULL comment '访问级别',
        `source_type` tinyint (4) NOT NULL DEFAULT '2' comment '所属主体, 2-Project, 3-Namespace',
        `expires_at` timestamp NOT NULL comment '过期时间',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_members_source_type_rid_uid` (`source_type`, `rid`, `uid`) LOCAL,
        KEY `idx_members_rid` (`rid`) LOCAL,
        KEY `idx_members_uid` (`uid`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '仓库成员表';

-- emails table
CREATE TABLE
    `emails` (
        `id` bigint (20) unsigned NOT NULL AUTO_INCREMENT comment '主键',
        `email` varchar(255) NOT NULL comment '用户邮箱',
        `uid` bigint (20) unsigned NOT NULL comment '用户 ID',
        `confirmation_token` char(64) NOT NULL comment '确认 Token',
        `confirmation_sent_at` timestamp NULL DEFAULT NULL COMMENT '确认邮件发送时间',
        `confirmed_at` timestamp NULL DEFAULT NULL COMMENT '确认时间',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_emails_email` (`email`) LOCAL,
        KEY `idx_emails_uid` (`uid`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '用户邮箱映射表';

CREATE TABLE
    `ssh_keys` (
        `id` bigint (20) NOT NULL AUTO_INCREMENT COMMENT '主键',
        `uid` bigint (20) unsigned NOT NULL DEFAULT '0' COMMENT '用户 ID',
        `content` text NOT NULL COMMENT '完整公钥',
        `title` varchar(255) NOT NULL COMMENT '标题',
        `type` tinyint (4) NOT NULL DEFAULT '0' COMMENT '公钥类型，0 用户公钥，1 部署公钥',
        `fingerprint` varchar(255) NOT NULL COMMENT '指纹',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_fingerprint` (`fingerprint`) LOCAL,
        KEY `idx_keys_uid` (`uid`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = 'SSH 公钥';

CREATE TABLE
    `deploy_keys_repositories` (
        `id` bigint (20) NOT NULL AUTO_INCREMENT COMMENT '主键',
        `kid` bigint (20) unsigned NOT NULL COMMENT '公钥 ID',
        `rid` bigint (20) unsigned NOT NULL COMMENT '存储库 ID',
        `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP comment '创建时间',
        `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment '修改时间',
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_deploy_keys_repositories_kid_and_rid` (`kid`, `rid`) LOCAL,
        KEY `idx_deploy_keys_repositories_rid` (`rid`) LOCAL
    ) DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '部署公钥开启项目';