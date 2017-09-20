-- MySQL dump 10.13  Distrib 5.7.18, for Linux (x86_64)
--
-- Host: localhost    Database: cattle
-- ------------------------------------------------------
-- Server version	5.7.18-0ubuntu0.16.04.1

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `DATABASECHANGELOG`
--

DROP TABLE IF EXISTS `DATABASECHANGELOG`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `DATABASECHANGELOG` (
  `ID` varchar(255) NOT NULL,
  `AUTHOR` varchar(255) NOT NULL,
  `FILENAME` varchar(255) NOT NULL,
  `DATEEXECUTED` datetime NOT NULL,
  `ORDEREXECUTED` int(11) NOT NULL,
  `EXECTYPE` varchar(10) NOT NULL,
  `MD5SUM` varchar(35) DEFAULT NULL,
  `DESCRIPTION` varchar(255) DEFAULT NULL,
  `COMMENTS` varchar(255) DEFAULT NULL,
  `TAG` varchar(255) DEFAULT NULL,
  `LIQUIBASE` varchar(20) DEFAULT NULL,
  `CONTEXTS` varchar(255) DEFAULT NULL,
  `LABELS` varchar(255) DEFAULT NULL,
  `DEPLOYMENT_ID` varchar(10) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `DATABASECHANGELOG`
--

LOCK TABLES `DATABASECHANGELOG` WRITE;
/*!40000 ALTER TABLE `DATABASECHANGELOG` DISABLE KEYS */;
INSERT INTO `DATABASECHANGELOG` VALUES ('dump1','rancher (generated)','db/core-200.xml','2017-09-14 00:44:16',1,'EXECUTED','7:6ae6b685df28e55ccd4697f56aee8b3c','createTable tableName=account; createTable tableName=agent; createTable tableName=audit_log; createTable tableName=auth_token; createTable tableName=catalog; createTable tableName=catalog_category; createTable tableName=catalog_file; createTable t...','',NULL,'3.5.3',NULL,NULL,'5375003739');
/*!40000 ALTER TABLE `DATABASECHANGELOG` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `DATABASECHANGELOGLOCK`
--

DROP TABLE IF EXISTS `DATABASECHANGELOGLOCK`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `DATABASECHANGELOGLOCK` (
  `ID` int(11) NOT NULL,
  `LOCKED` bit(1) NOT NULL,
  `LOCKGRANTED` datetime DEFAULT NULL,
  `LOCKEDBY` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`ID`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `DATABASECHANGELOGLOCK`
--

LOCK TABLES `DATABASECHANGELOGLOCK` WRITE;
/*!40000 ALTER TABLE `DATABASECHANGELOGLOCK` DISABLE KEYS */;
INSERT INTO `DATABASECHANGELOGLOCK` VALUES (1,'\0',NULL,NULL);
/*!40000 ALTER TABLE `DATABASECHANGELOGLOCK` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `account`
--

DROP TABLE IF EXISTS `account`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `account` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `external_id` varchar(255) DEFAULT NULL,
  `external_id_type` varchar(128) DEFAULT NULL,
  `version` varchar(128) DEFAULT NULL,
  `cluster_id` bigint(20) DEFAULT NULL,
  `cluster_owner` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_account_uuid` (`uuid`),
  KEY `fk_account__cluster_id` (`cluster_id`),
  KEY `idx_account_name` (`name`),
  KEY `idx_account_remove_time` (`remove_time`),
  KEY `idx_account_removed` (`removed`),
  KEY `idx_account_state` (`state`),
  KEY `idx_external_ids` (`external_id`,`external_id_type`),
  CONSTRAINT `fk_account__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `account`
--

LOCK TABLES `account` WRITE;
/*!40000 ALTER TABLE `account` DISABLE KEYS */;
/*!40000 ALTER TABLE `account` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `agent`
--

DROP TABLE IF EXISTS `agent`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `agent` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `uri` varchar(255) DEFAULT NULL,
  `resource_account_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_agent_uuid` (`uuid`),
  KEY `fk_agent__account_id` (`account_id`),
  KEY `fk_agent__cluster_id` (`cluster_id`),
  KEY `fk_agent__resource_account_id` (`resource_account_id`),
  KEY `fk_agent__uri` (`uri`),
  KEY `idx_agent_name` (`name`),
  KEY `idx_agent_remove_time` (`remove_time`),
  KEY `idx_agent_removed` (`removed`),
  KEY `idx_agent_state` (`state`),
  CONSTRAINT `fk_agent__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_agent__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_agent__resource_account_id` FOREIGN KEY (`resource_account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `agent`
--

LOCK TABLES `agent` WRITE;
/*!40000 ALTER TABLE `agent` DISABLE KEYS */;
/*!40000 ALTER TABLE `agent` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `audit_log`
--

DROP TABLE IF EXISTS `audit_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `audit_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` bigint(20) DEFAULT NULL,
  `authenticated_as_account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `auth_type` varchar(255) DEFAULT NULL,
  `event_type` varchar(255) NOT NULL,
  `resource_type` varchar(255) NOT NULL,
  `resource_id` bigint(20) DEFAULT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `created` datetime DEFAULT NULL,
  `data` mediumtext,
  `authenticated_as_identity_id` varchar(255) DEFAULT NULL,
  `runtime` bigint(20) DEFAULT NULL,
  `client_ip` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_audit_log__account_id` (`account_id`),
  KEY `fk_audit_log__authenticated_as_account_id` (`authenticated_as_account_id`),
  KEY `idx_audit_log_client_ip` (`client_ip`),
  KEY `idx_audit_log_created` (`created`),
  KEY `idx_audit_log_event_type` (`event_type`),
  CONSTRAINT `fk_audit_log__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_audit_log__authenticated_as_account_id` FOREIGN KEY (`authenticated_as_account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `audit_log`
--

LOCK TABLES `audit_log` WRITE;
/*!40000 ALTER TABLE `audit_log` DISABLE KEYS */;
/*!40000 ALTER TABLE `audit_log` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `auth_token`
--

DROP TABLE IF EXISTS `auth_token`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `auth_token` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` bigint(20) NOT NULL,
  `created` datetime NOT NULL,
  `expires` datetime NOT NULL,
  `key` varchar(40) NOT NULL,
  `value` mediumtext NOT NULL,
  `version` varchar(255) NOT NULL,
  `provider` varchar(255) NOT NULL,
  `authenticated_as_account_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_auth_token_key` (`key`),
  UNIQUE KEY `key` (`key`),
  KEY `auth_token_ibfk_1` (`authenticated_as_account_id`),
  KEY `fk_auth_token__account_id` (`account_id`),
  KEY `idx_auth_token_expires` (`expires`),
  CONSTRAINT `auth_token_ibfk_1` FOREIGN KEY (`authenticated_as_account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_auth_token__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `auth_token`
--

LOCK TABLES `auth_token` WRITE;
/*!40000 ALTER TABLE `auth_token` DISABLE KEYS */;
/*!40000 ALTER TABLE `auth_token` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog`
--

DROP TABLE IF EXISTS `catalog`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `environment_id` varchar(255) DEFAULT NULL,
  `name` varchar(1024) DEFAULT NULL,
  `url` varchar(1024) DEFAULT NULL,
  `branch` varchar(1024) DEFAULT NULL,
  `commit` varchar(1024) DEFAULT NULL,
  `type` varchar(255) DEFAULT NULL,
  `kind` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_catalog_environment_id` (`environment_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog`
--

LOCK TABLES `catalog` WRITE;
/*!40000 ALTER TABLE `catalog` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_category`
--

DROP TABLE IF EXISTS `catalog_category`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_category` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `name` varchar(1024) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_category`
--

LOCK TABLES `catalog_category` WRITE;
/*!40000 ALTER TABLE `catalog_category` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_category` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_file`
--

DROP TABLE IF EXISTS `catalog_file`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_file` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `version_id` bigint(20) DEFAULT NULL,
  `name` varchar(1024) DEFAULT NULL,
  `contents` mediumblob,
  PRIMARY KEY (`id`),
  KEY `fk_catalog_file__version_id` (`version_id`),
  CONSTRAINT `fk_catalog_file__version_id` FOREIGN KEY (`version_id`) REFERENCES `catalog_version` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_file`
--

LOCK TABLES `catalog_file` WRITE;
/*!40000 ALTER TABLE `catalog_file` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_file` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_label`
--

DROP TABLE IF EXISTS `catalog_label`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_label` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `template_id` bigint(20) DEFAULT NULL,
  `key` varchar(1024) DEFAULT NULL,
  `value` varchar(1024) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_catalog_label__template_id` (`template_id`),
  CONSTRAINT `fk_catalog_label__template_id` FOREIGN KEY (`template_id`) REFERENCES `catalog_template` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_label`
--

LOCK TABLES `catalog_label` WRITE;
/*!40000 ALTER TABLE `catalog_label` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_label` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_template`
--

DROP TABLE IF EXISTS `catalog_template`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_template` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `environment_id` varchar(255) DEFAULT NULL,
  `catalog_id` bigint(20) DEFAULT NULL,
  `name` varchar(1024) DEFAULT NULL,
  `is_system` varchar(255) DEFAULT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `default_version` varchar(1024) DEFAULT NULL,
  `path` varchar(1024) DEFAULT NULL,
  `maintainer` varchar(1024) DEFAULT NULL,
  `license` mediumtext,
  `project_url` varchar(1024) DEFAULT NULL,
  `upgrade_from` varchar(1024) DEFAULT NULL,
  `folder_name` varchar(1024) DEFAULT NULL,
  `catalog` varchar(1024) DEFAULT NULL,
  `base` varchar(1024) DEFAULT NULL,
  `icon` mediumtext,
  `icon_filename` varchar(255) DEFAULT NULL,
  `readme` mediumblob,
  PRIMARY KEY (`id`),
  KEY `fk_catalog_template__catalog_id` (`catalog_id`),
  KEY `idx_catalog_template_environment_id` (`environment_id`),
  CONSTRAINT `fk_catalog_template__catalog_id` FOREIGN KEY (`catalog_id`) REFERENCES `catalog` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_template`
--

LOCK TABLES `catalog_template` WRITE;
/*!40000 ALTER TABLE `catalog_template` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_template` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_template_category`
--

DROP TABLE IF EXISTS `catalog_template_category`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_template_category` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `template_id` bigint(20) DEFAULT NULL,
  `category_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_catalog_t_catalog__category_id` (`category_id`),
  KEY `fk_catalog_t_category__template_id` (`template_id`),
  CONSTRAINT `fk_catalog_t_catalog__category_id` FOREIGN KEY (`category_id`) REFERENCES `catalog_category` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION,
  CONSTRAINT `fk_catalog_t_category__template_id` FOREIGN KEY (`template_id`) REFERENCES `catalog_template` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_template_category`
--

LOCK TABLES `catalog_template_category` WRITE;
/*!40000 ALTER TABLE `catalog_template_category` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_template_category` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_version`
--

DROP TABLE IF EXISTS `catalog_version`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_version` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `template_id` bigint(20) DEFAULT NULL,
  `revision` bigint(20) DEFAULT NULL,
  `version` varchar(1024) DEFAULT NULL,
  `minimum_rancher_version` varchar(1024) DEFAULT NULL,
  `maximum_rancher_version` varchar(1024) DEFAULT NULL,
  `upgrade_from` varchar(1024) DEFAULT NULL,
  `readme` mediumblob,
  PRIMARY KEY (`id`),
  KEY `fk_catalog_template__template_id` (`template_id`),
  CONSTRAINT `fk_catalog_template__template_id` FOREIGN KEY (`template_id`) REFERENCES `catalog_template` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_version`
--

LOCK TABLES `catalog_version` WRITE;
/*!40000 ALTER TABLE `catalog_version` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_version` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `catalog_version_label`
--

DROP TABLE IF EXISTS `catalog_version_label`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `catalog_version_label` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `version_id` bigint(20) DEFAULT NULL,
  `key` varchar(1024) DEFAULT NULL,
  `value` varchar(1024) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_catalog_v_l__version_id` (`version_id`),
  CONSTRAINT `fk_catalog_v_l__version_id` FOREIGN KEY (`version_id`) REFERENCES `catalog_version` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `catalog_version_label`
--

LOCK TABLES `catalog_version_label` WRITE;
/*!40000 ALTER TABLE `catalog_version_label` DISABLE KEYS */;
/*!40000 ALTER TABLE `catalog_version_label` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `certificate`
--

DROP TABLE IF EXISTS `certificate`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `certificate` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `cert_chain` text,
  `cert` text,
  `key` text,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cert_data_uuid` (`uuid`),
  KEY `fk_cert_data__account_id` (`account_id`),
  KEY `fk_certificate__creator_id` (`creator_id`),
  KEY `idx_cert_data_name` (`name`),
  KEY `idx_cert_data_remove_time` (`remove_time`),
  KEY `idx_cert_data_removed` (`removed`),
  KEY `idx_cert_data_state` (`state`),
  CONSTRAINT `fk_cert_data__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_certificate__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `certificate`
--

LOCK TABLES `certificate` WRITE;
/*!40000 ALTER TABLE `certificate` DISABLE KEYS */;
/*!40000 ALTER TABLE `certificate` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `cluster`
--

DROP TABLE IF EXISTS `cluster`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `cluster` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `embedded` bit(1) NOT NULL DEFAULT b'0',
  `creator_id` bigint(20) DEFAULT NULL,
  `default_network_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_uuid` (`uuid`),
  KEY `fk_cluster__creator_id` (`creator_id`),
  KEY `fk_cluster__network_id` (`default_network_id`),
  KEY `idx_cluster_name` (`name`),
  KEY `idx_cluster_remove_time` (`remove_time`),
  KEY `idx_cluster_removed` (`removed`),
  KEY `idx_cluster_state` (`state`),
  CONSTRAINT `fk_cluster__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_cluster__network_id` FOREIGN KEY (`default_network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `cluster`
--

LOCK TABLES `cluster` WRITE;
/*!40000 ALTER TABLE `cluster` DISABLE KEYS */;
/*!40000 ALTER TABLE `cluster` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `credential`
--

DROP TABLE IF EXISTS `credential`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `credential` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `public_value` varchar(4096) DEFAULT NULL,
  `secret_value` varchar(4096) DEFAULT NULL,
  `registry_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_credential_uuid` (`uuid`),
  KEY `fk_credential__account_id` (`account_id`),
  KEY `fk_credential__registry_id` (`registry_id`),
  KEY `idx_credential_name` (`name`),
  KEY `idx_credential_remove_time` (`remove_time`),
  KEY `idx_credential_removed` (`removed`),
  KEY `idx_credential_state` (`state`),
  CONSTRAINT `fk_credential__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_credential__registry_id` FOREIGN KEY (`registry_id`) REFERENCES `storage_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `credential`
--

LOCK TABLES `credential` WRITE;
/*!40000 ALTER TABLE `credential` DISABLE KEYS */;
/*!40000 ALTER TABLE `credential` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `data`
--

DROP TABLE IF EXISTS `data`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `data` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `visible` bit(1) NOT NULL DEFAULT b'1',
  `value` text NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_data_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `data`
--

LOCK TABLES `data` WRITE;
/*!40000 ALTER TABLE `data` DISABLE KEYS */;
/*!40000 ALTER TABLE `data` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `deployment_unit`
--

DROP TABLE IF EXISTS `deployment_unit`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `deployment_unit` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` text,
  `service_index` varchar(255) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  `environment_id` bigint(20) NOT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  `requested_revision_id` bigint(20) DEFAULT NULL,
  `revision_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_deployment_unit_uuid` (`uuid`),
  KEY `fk_deployment_unit__account_id` (`account_id`),
  KEY `fk_deployment_unit__cluster_id` (`cluster_id`),
  KEY `fk_deployment_unit__environment_id` (`environment_id`),
  KEY `fk_deployment_unit__host_id` (`host_id`),
  KEY `fk_deployment_unit__revision_id` (`revision_id`),
  KEY `fk_deployment_unit__service_id` (`service_id`),
  KEY `fk_deployment_unit_requested_revision_id` (`requested_revision_id`),
  KEY `idx_deployment_unit__external_id` (`external_id`),
  KEY `idx_deployment_unit_name` (`name`),
  KEY `idx_deployment_unit_remove_time` (`remove_time`),
  KEY `idx_deployment_unit_removed` (`removed`),
  KEY `idx_deployment_unit_state` (`state`),
  CONSTRAINT `fk_deployment_unit__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit__revision_id` FOREIGN KEY (`revision_id`) REFERENCES `revision` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit_requested_revision_id` FOREIGN KEY (`requested_revision_id`) REFERENCES `revision` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `deployment_unit`
--

LOCK TABLES `deployment_unit` WRITE;
/*!40000 ALTER TABLE `deployment_unit` DISABLE KEYS */;
/*!40000 ALTER TABLE `deployment_unit` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `dynamic_schema`
--

DROP TABLE IF EXISTS `dynamic_schema`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `dynamic_schema` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `data` text,
  `parent` varchar(255) DEFAULT NULL,
  `definition` mediumtext,
  `service_id` bigint(20) DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_dynamic_schema_uuid` (`uuid`),
  KEY `fk_dynamic_schema__account_id` (`account_id`),
  KEY `fk_dynamic_schema__creator_id` (`creator_id`),
  KEY `fk_dynamic_schema__service_id` (`service_id`),
  KEY `idx_dynamic_schema_name` (`name`),
  KEY `idx_dynamic_schema_removed` (`removed`),
  KEY `idx_dynamic_schema_state` (`state`),
  CONSTRAINT `fk_dynamic_schema__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_dynamic_schema__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_dynamic_schema__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `dynamic_schema`
--

LOCK TABLES `dynamic_schema` WRITE;
/*!40000 ALTER TABLE `dynamic_schema` DISABLE KEYS */;
/*!40000 ALTER TABLE `dynamic_schema` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `dynamic_schema_role`
--

DROP TABLE IF EXISTS `dynamic_schema_role`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `dynamic_schema_role` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `dynamic_schema_id` bigint(20) DEFAULT NULL,
  `role` varchar(255) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_dynamic_schema_role_dynamic_schema_id` (`dynamic_schema_id`),
  CONSTRAINT `fk_dynamic_schema_role_dynamic_schema_id` FOREIGN KEY (`dynamic_schema_id`) REFERENCES `dynamic_schema` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `dynamic_schema_role`
--

LOCK TABLES `dynamic_schema_role` WRITE;
/*!40000 ALTER TABLE `dynamic_schema_role` DISABLE KEYS */;
/*!40000 ALTER TABLE `dynamic_schema_role` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `external_event`
--

DROP TABLE IF EXISTS `external_event`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `external_event` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `data` text,
  `external_id` varchar(255) DEFAULT NULL,
  `event_type` varchar(255) DEFAULT NULL,
  `reported_account_id` bigint(20) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_external_event_uuid` (`uuid`),
  KEY `fk_external_event__account_id` (`account_id`),
  KEY `fk_external_event__cluster_id` (`cluster_id`),
  KEY `fk_external_event__creator_id` (`creator_id`),
  KEY `fk_external_event__reported_account_id` (`reported_account_id`),
  KEY `idx_external_event_state` (`state`),
  CONSTRAINT `fk_external_event__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_external_event__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_external_event__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_external_event__reported_account_id` FOREIGN KEY (`reported_account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `external_event`
--

LOCK TABLES `external_event` WRITE;
/*!40000 ALTER TABLE `external_event` DISABLE KEYS */;
/*!40000 ALTER TABLE `external_event` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `generic_object`
--

DROP TABLE IF EXISTS `generic_object`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `generic_object` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `key` varchar(255) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_generic_object_uuid` (`uuid`),
  KEY `fk_generic_object__account_id` (`account_id`),
  KEY `fk_generic_object__cluster_id` (`cluster_id`),
  KEY `fk_generic_object__creator_id` (`creator_id`),
  KEY `idx_generic_object_key` (`key`),
  KEY `idx_generic_object_name` (`name`),
  KEY `idx_generic_object_remove_time` (`remove_time`),
  KEY `idx_generic_object_removed` (`removed`),
  KEY `idx_generic_object_state` (`state`),
  CONSTRAINT `fk_generic_object__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_generic_object__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_generic_object__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `generic_object`
--

LOCK TABLES `generic_object` WRITE;
/*!40000 ALTER TABLE `generic_object` DISABLE KEYS */;
/*!40000 ALTER TABLE `generic_object` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ha_membership`
--

DROP TABLE IF EXISTS `ha_membership`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ha_membership` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `uuid` varchar(128) NOT NULL,
  `heartbeat` bigint(20) DEFAULT NULL,
  `config` mediumtext,
  `clustered` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_membership_uuid` (`uuid`),
  KEY `idx_cluster_membership_name` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ha_membership`
--

LOCK TABLES `ha_membership` WRITE;
/*!40000 ALTER TABLE `ha_membership` DISABLE KEYS */;
/*!40000 ALTER TABLE `ha_membership` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `host`
--

DROP TABLE IF EXISTS `host`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `host` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `uri` varchar(255) DEFAULT NULL,
  `agent_id` bigint(20) DEFAULT NULL,
  `agent_state` varchar(128) DEFAULT NULL,
  `local_storage_mb` bigint(20) DEFAULT NULL,
  `memory` bigint(20) DEFAULT NULL,
  `milli_cpu` bigint(20) DEFAULT NULL,
  `environment_id` bigint(20) DEFAULT NULL,
  `remove_after` datetime DEFAULT NULL,
  `host_template_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT NULL,
  `revision` bigint(20) NOT NULL DEFAULT '0',
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_host_uuid` (`uuid`),
  KEY `fk_host__agent_id` (`agent_id`),
  KEY `fk_host__cluster_id` (`cluster_id`),
  KEY `fk_host__creator_id` (`creator_id`),
  KEY `fk_host__environment_id` (`environment_id`),
  KEY `fk_host__host_template_id` (`host_template_id`),
  KEY `idx_host__external_id` (`external_id`),
  KEY `idx_host__remove_after` (`remove_after`),
  KEY `idx_host_name` (`name`),
  KEY `idx_host_remove_time` (`remove_time`),
  KEY `idx_host_removed` (`removed`),
  KEY `idx_host_state` (`state`),
  CONSTRAINT `fk_host__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__host_template_id` FOREIGN KEY (`host_template_id`) REFERENCES `host_template` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `host`
--

LOCK TABLES `host` WRITE;
/*!40000 ALTER TABLE `host` DISABLE KEYS */;
/*!40000 ALTER TABLE `host` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `host_template`
--

DROP TABLE IF EXISTS `host_template`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `host_template` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `driver` varchar(255) DEFAULT NULL,
  `flavor_prefix` varchar(255) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_host_template_uuid` (`uuid`),
  KEY `fk_host_template__cluster_id` (`cluster_id`),
  KEY `fk_host_template__creator_id` (`creator_id`),
  KEY `idx_host_template_name` (`name`),
  KEY `idx_host_template_remove_time` (`remove_time`),
  KEY `idx_host_template_removed` (`removed`),
  KEY `idx_host_template_state` (`state`),
  CONSTRAINT `fk_host_template__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host_template__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `host_template`
--

LOCK TABLES `host_template` WRITE;
/*!40000 ALTER TABLE `host_template` DISABLE KEYS */;
/*!40000 ALTER TABLE `host_template` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `instance`
--

DROP TABLE IF EXISTS `instance`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `instance` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `memory_mb` bigint(20) DEFAULT NULL,
  `hostname` varchar(255) DEFAULT NULL,
  `instance_triggered_stop` varchar(128) DEFAULT NULL,
  `agent_id` bigint(20) DEFAULT NULL,
  `domain` varchar(128) DEFAULT NULL,
  `first_running` datetime DEFAULT NULL,
  `token` varchar(255) DEFAULT NULL,
  `userdata` text,
  `registry_credential_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT NULL,
  `native_container` bit(1) NOT NULL DEFAULT b'0',
  `network_container_id` bigint(20) DEFAULT NULL,
  `health_state` varchar(128) DEFAULT NULL,
  `start_count` bigint(20) DEFAULT NULL,
  `create_index` bigint(20) DEFAULT NULL,
  `version` varchar(255) DEFAULT '0',
  `memory_reservation` bigint(20) DEFAULT NULL,
  `milli_cpu_reservation` bigint(20) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  `environment_id` bigint(20) DEFAULT NULL,
  `deployment_unit_id` bigint(20) DEFAULT NULL,
  `revision_id` bigint(20) DEFAULT NULL,
  `desired` bit(1) NOT NULL DEFAULT b'1',
  `host_id` bigint(20) DEFAULT NULL,
  `network_id` bigint(20) DEFAULT NULL,
  `service_index` int(11) DEFAULT NULL,
  `upgrade_time` datetime DEFAULT NULL,
  `revision` bigint(20) NOT NULL DEFAULT '0',
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_instance_uuid` (`uuid`),
  KEY `fk_instance__account_id` (`account_id`),
  KEY `fk_instance__agent_id` (`agent_id`),
  KEY `fk_instance__cluster_id` (`cluster_id`),
  KEY `fk_instance__creator_id` (`creator_id`),
  KEY `fk_instance__deployment_unit_id` (`deployment_unit_id`),
  KEY `fk_instance__environment_id` (`environment_id`),
  KEY `fk_instance__host_id` (`host_id`),
  KEY `fk_instance__instance_id` (`network_container_id`),
  KEY `fk_instance__network_id` (`network_id`),
  KEY `fk_instance__registry_credential_id` (`registry_credential_id`),
  KEY `fk_instance__revision_id` (`revision_id`),
  KEY `fk_instance__service_id` (`service_id`),
  KEY `idx_instance_external_id` (`external_id`),
  KEY `idx_instance_name` (`name`),
  KEY `idx_instance_remove_time` (`remove_time`),
  KEY `idx_instance_removed` (`removed`),
  KEY `idx_instance_state` (`state`),
  CONSTRAINT `fk_instance__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__deployment_unit_id` FOREIGN KEY (`deployment_unit_id`) REFERENCES `deployment_unit` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__instance_id` FOREIGN KEY (`network_container_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__registry_credential_id` FOREIGN KEY (`registry_credential_id`) REFERENCES `credential` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__revision_id` FOREIGN KEY (`revision_id`) REFERENCES `revision` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `instance`
--

LOCK TABLES `instance` WRITE;
/*!40000 ALTER TABLE `instance` DISABLE KEYS */;
/*!40000 ALTER TABLE `instance` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `key_value`
--

DROP TABLE IF EXISTS `key_value`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `key_value` (
  `name` varchar(255) DEFAULT NULL,
  `value` mediumblob,
  `revision` bigint(20) DEFAULT NULL,
  UNIQUE KEY `uix_key_value_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `key_value`
--

LOCK TABLES `key_value` WRITE;
/*!40000 ALTER TABLE `key_value` DISABLE KEYS */;
/*!40000 ALTER TABLE `key_value` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `machine_driver`
--

DROP TABLE IF EXISTS `machine_driver`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `machine_driver` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` text,
  `uri` varchar(255) DEFAULT NULL,
  `md5checksum` varchar(255) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_machine_driver_uuid` (`uuid`),
  KEY `fk_machine_driver__creator_id` (`creator_id`),
  KEY `idx_machine_driver_name` (`name`),
  KEY `idx_machine_driver_remove_time` (`remove_time`),
  KEY `idx_machine_driver_removed` (`removed`),
  KEY `idx_machine_driver_state` (`state`),
  CONSTRAINT `fk_machine_driver__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `machine_driver`
--

LOCK TABLES `machine_driver` WRITE;
/*!40000 ALTER TABLE `machine_driver` DISABLE KEYS */;
/*!40000 ALTER TABLE `machine_driver` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `mount`
--

DROP TABLE IF EXISTS `mount`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `mount` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `volume_id` bigint(20) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `permissions` varchar(128) DEFAULT NULL,
  `path` varchar(512) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_mount_uuid` (`uuid`),
  KEY `fk_mount__account_id` (`account_id`),
  KEY `fk_mount__instance_id` (`instance_id`),
  KEY `fk_mount__volume_id` (`volume_id`),
  KEY `idx_mount_name` (`name`),
  KEY `idx_mount_remove_time` (`remove_time`),
  KEY `idx_mount_removed` (`removed`),
  KEY `idx_mount_state` (`state`),
  CONSTRAINT `fk_mount__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_mount__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_mount__volume_id` FOREIGN KEY (`volume_id`) REFERENCES `volume` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `mount`
--

LOCK TABLES `mount` WRITE;
/*!40000 ALTER TABLE `mount` DISABLE KEYS */;
/*!40000 ALTER TABLE `mount` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `network`
--

DROP TABLE IF EXISTS `network`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `network` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `domain` varchar(128) DEFAULT NULL,
  `network_driver_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_uuid` (`uuid`),
  KEY `fk_network__cluster_id` (`cluster_id`),
  KEY `fk_network__network_driver_id` (`network_driver_id`),
  KEY `idx_network_name` (`name`),
  KEY `idx_network_remove_time` (`remove_time`),
  KEY `idx_network_removed` (`removed`),
  KEY `idx_network_state` (`state`),
  CONSTRAINT `fk_network__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network__network_driver_id` FOREIGN KEY (`network_driver_id`) REFERENCES `network_driver` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `network`
--

LOCK TABLES `network` WRITE;
/*!40000 ALTER TABLE `network` DISABLE KEYS */;
/*!40000 ALTER TABLE `network` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `network_driver`
--

DROP TABLE IF EXISTS `network_driver`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `network_driver` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `service_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_driver_uuid` (`uuid`),
  KEY `fk_network_driver__cluster_id` (`cluster_id`),
  KEY `fk_network_driver__service_id` (`service_id`),
  KEY `idx_network_driver_name` (`name`),
  KEY `idx_network_driver_remove_time` (`remove_time`),
  KEY `idx_network_driver_removed` (`removed`),
  KEY `idx_network_driver_state` (`state`),
  CONSTRAINT `fk_network_driver__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network_driver__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `network_driver`
--

LOCK TABLES `network_driver` WRITE;
/*!40000 ALTER TABLE `network_driver` DISABLE KEYS */;
/*!40000 ALTER TABLE `network_driver` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `process_execution`
--

DROP TABLE IF EXISTS `process_execution`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `process_execution` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `process_instance_id` bigint(20) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `log` mediumtext,
  `created` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_process_execution__uuid` (`uuid`),
  KEY `fk_process_execution_process_instance_id` (`process_instance_id`),
  KEY `idx_processs_execution_created_time` (`created`),
  CONSTRAINT `fk_process_execution_process_instance_id` FOREIGN KEY (`process_instance_id`) REFERENCES `process_instance` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `process_execution`
--

LOCK TABLES `process_execution` WRITE;
/*!40000 ALTER TABLE `process_execution` DISABLE KEYS */;
/*!40000 ALTER TABLE `process_execution` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `process_instance`
--

DROP TABLE IF EXISTS `process_instance`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `process_instance` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `start_time` datetime DEFAULT NULL,
  `end_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `priority` int(11) DEFAULT '0',
  `process_name` varchar(128) DEFAULT NULL,
  `resource_type` varchar(128) DEFAULT NULL,
  `resource_id` varchar(128) DEFAULT NULL,
  `result` varchar(128) DEFAULT NULL,
  `exit_reason` varchar(128) DEFAULT NULL,
  `phase` varchar(128) DEFAULT NULL,
  `start_process_server_id` varchar(128) DEFAULT NULL,
  `running_process_server_id` varchar(128) DEFAULT NULL,
  `execution_count` bigint(20) NOT NULL DEFAULT '0',
  `run_after` datetime DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_process_instance__account_id` (`account_id`),
  KEY `fk_process_instance__cluster_id` (`cluster_id`),
  KEY `idx_process_instance_end_time` (`end_time`),
  KEY `idx_process_instance_et_rt_ri` (`end_time`,`resource_type`,`resource_id`),
  KEY `idx_process_instance_priority` (`priority`),
  KEY `idx_process_instance_run_after` (`run_after`),
  KEY `idx_process_instance_start_time` (`start_time`),
  CONSTRAINT `fk_process_instance__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_process_instance__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `process_instance`
--

LOCK TABLES `process_instance` WRITE;
/*!40000 ALTER TABLE `process_instance` DISABLE KEYS */;
/*!40000 ALTER TABLE `process_instance` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `project_member`
--

DROP TABLE IF EXISTS `project_member`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `project_member` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `external_id` varchar(255) NOT NULL,
  `project_id` bigint(20) NOT NULL,
  `external_id_type` varchar(255) NOT NULL,
  `role` varchar(255) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_project_member_uuid` (`uuid`),
  KEY `fk_project_member__account_id` (`account_id`),
  KEY `fk_project_member__project_id` (`project_id`),
  KEY `idx_project_member_name` (`name`),
  KEY `idx_project_member_remove_time` (`remove_time`),
  KEY `idx_project_member_removed` (`removed`),
  KEY `idx_project_member_state` (`state`),
  CONSTRAINT `fk_project_member__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_project_member__project_id` FOREIGN KEY (`project_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `project_member`
--

LOCK TABLES `project_member` WRITE;
/*!40000 ALTER TABLE `project_member` DISABLE KEYS */;
/*!40000 ALTER TABLE `project_member` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `resource_pool`
--

DROP TABLE IF EXISTS `resource_pool`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `resource_pool` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `pool_type` varchar(255) DEFAULT NULL,
  `pool_id` bigint(20) DEFAULT NULL,
  `item` varchar(255) DEFAULT NULL,
  `owner_type` varchar(255) DEFAULT NULL,
  `owner_id` bigint(20) DEFAULT NULL,
  `qualifier` varchar(128) NOT NULL DEFAULT 'default',
  `sub_owner` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_resource_pool_uuid` (`uuid`),
  UNIQUE KEY `idx_pool_item2` (`pool_type`,`pool_id`,`qualifier`,`item`),
  KEY `fk_resource_pool__account_id` (`account_id`),
  KEY `idx_pool_owner2` (`pool_type`,`pool_id`,`qualifier`,`owner_type`,`owner_id`,`sub_owner`),
  KEY `idx_resource_pool_name` (`name`),
  KEY `idx_resource_pool_remove_time` (`remove_time`),
  KEY `idx_resource_pool_removed` (`removed`),
  KEY `idx_resource_pool_state` (`state`),
  CONSTRAINT `fk_resource_pool__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `resource_pool`
--

LOCK TABLES `resource_pool` WRITE;
/*!40000 ALTER TABLE `resource_pool` DISABLE KEYS */;
/*!40000 ALTER TABLE `resource_pool` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `revision`
--

DROP TABLE IF EXISTS `revision`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `revision` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `service_id` bigint(20) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_revision_uuid` (`uuid`),
  KEY `fk_revision__account_id` (`account_id`),
  KEY `fk_revision__creator_id` (`creator_id`),
  KEY `fk_revision__service_id` (`service_id`),
  KEY `idx_revision_name` (`name`),
  KEY `idx_revision_remove_time` (`remove_time`),
  KEY `idx_revision_removed` (`removed`),
  KEY `idx_revision_state` (`state`),
  CONSTRAINT `fk_revision__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_revision__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_revision__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `revision`
--

LOCK TABLES `revision` WRITE;
/*!40000 ALTER TABLE `revision` DISABLE KEYS */;
/*!40000 ALTER TABLE `revision` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `scheduled_upgrade`
--

DROP TABLE IF EXISTS `scheduled_upgrade`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `scheduled_upgrade` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `run_after` datetime DEFAULT NULL,
  `data` mediumtext,
  `environment_id` bigint(20) DEFAULT NULL,
  `started` datetime DEFAULT NULL,
  `finished` datetime DEFAULT NULL,
  `priority` bigint(20) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_scheduled_upgrade_uuid` (`uuid`),
  KEY `fk_scheduled_upgrade__account_id` (`account_id`),
  KEY `fk_scheduled_upgrade__environment_id` (`environment_id`),
  CONSTRAINT `fk_scheduled_upgrade__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_scheduled_upgrade__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `scheduled_upgrade`
--

LOCK TABLES `scheduled_upgrade` WRITE;
/*!40000 ALTER TABLE `scheduled_upgrade` DISABLE KEYS */;
/*!40000 ALTER TABLE `scheduled_upgrade` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `secret`
--

DROP TABLE IF EXISTS `secret`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `secret` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `value` mediumtext,
  `environment_id` bigint(20) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_secret_uuid` (`uuid`),
  KEY `fk_secret__account_id` (`account_id`),
  KEY `fk_secret__creator_id` (`creator_id`),
  KEY `fk_secret__environment_id` (`environment_id`),
  KEY `idx_secret_name` (`name`),
  KEY `idx_secret_remove_time` (`remove_time`),
  KEY `idx_secret_removed` (`removed`),
  KEY `idx_secret_state` (`state`),
  CONSTRAINT `fk_secret__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_secret__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_secret__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `secret`
--

LOCK TABLES `secret` WRITE;
/*!40000 ALTER TABLE `secret` DISABLE KEYS */;
/*!40000 ALTER TABLE `secret` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `service`
--

DROP TABLE IF EXISTS `service`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `service` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `environment_id` bigint(20) DEFAULT NULL,
  `vip` varchar(255) DEFAULT NULL,
  `create_index` bigint(20) DEFAULT NULL,
  `selector` varchar(4096) DEFAULT NULL,
  `external_id` varchar(255) DEFAULT NULL,
  `health_state` varchar(128) DEFAULT NULL,
  `previous_revision_id` bigint(20) DEFAULT NULL,
  `revision_id` bigint(20) DEFAULT NULL,
  `revision` bigint(20) NOT NULL DEFAULT '0',
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_uuid` (`uuid`),
  KEY `fk_service__account_id` (`account_id`),
  KEY `fk_service__creator_id` (`creator_id`),
  KEY `fk_service__environment_id` (`environment_id`),
  KEY `fk_service__previous_revision_id` (`previous_revision_id`),
  KEY `fk_service__revision_id` (`revision_id`),
  KEY `fk_service_cluster_id` (`cluster_id`),
  KEY `idx_service_external_id` (`external_id`),
  KEY `idx_service_name` (`name`),
  KEY `idx_service_remove_time` (`remove_time`),
  KEY `idx_service_removed` (`removed`),
  KEY `idx_service_state` (`state`),
  CONSTRAINT `fk_service__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service__previous_revision_id` FOREIGN KEY (`previous_revision_id`) REFERENCES `revision` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION,
  CONSTRAINT `fk_service__revision_id` FOREIGN KEY (`revision_id`) REFERENCES `revision` (`id`) ON DELETE CASCADE ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service`
--

LOCK TABLES `service` WRITE;
/*!40000 ALTER TABLE `service` DISABLE KEYS */;
/*!40000 ALTER TABLE `service` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `service_event`
--

DROP TABLE IF EXISTS `service_event`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `service_event` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `host_id` bigint(20) DEFAULT NULL,
  `healthcheck_uuid` varchar(255) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `reported_health` varchar(255) DEFAULT NULL,
  `external_timestamp` bigint(20) DEFAULT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_event_uuid` (`uuid`),
  KEY `fk_service_event__account_id` (`account_id`),
  KEY `fk_service_event__creator_id` (`creator_id`),
  KEY `fk_service_event__host_id` (`host_id`),
  KEY `fk_service_event__instance_id` (`instance_id`),
  KEY `idx_service_event_name` (`name`),
  KEY `idx_service_event_remove_time` (`remove_time`),
  KEY `idx_service_event_removed` (`removed`),
  KEY `idx_service_event_state` (`state`),
  CONSTRAINT `fk_service_event__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_event__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_event__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_event__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_event`
--

LOCK TABLES `service_event` WRITE;
/*!40000 ALTER TABLE `service_event` DISABLE KEYS */;
/*!40000 ALTER TABLE `service_event` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `service_log`
--

DROP TABLE IF EXISTS `service_log`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `service_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `created` datetime DEFAULT NULL,
  `data` text,
  `end_time` datetime DEFAULT NULL,
  `event_type` varchar(255) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `transaction_id` varchar(255) DEFAULT NULL,
  `sub_log` bit(1) NOT NULL DEFAULT b'0',
  `level` varchar(255) DEFAULT NULL,
  `deployment_unit_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_service_log__account_id` (`account_id`),
  KEY `fk_service_log__deployment_unit_id` (`deployment_unit_id`),
  KEY `fk_service_log__instance_id` (`instance_id`),
  KEY `fk_service_log__service_id` (`service_id`),
  CONSTRAINT `fk_service_log__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_log__deployment_unit_id` FOREIGN KEY (`deployment_unit_id`) REFERENCES `deployment_unit` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_log__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_log__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_log`
--

LOCK TABLES `service_log` WRITE;
/*!40000 ALTER TABLE `service_log` DISABLE KEYS */;
/*!40000 ALTER TABLE `service_log` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `setting`
--

DROP TABLE IF EXISTS `setting`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `setting` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `value` mediumtext NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_setting_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `setting`
--

LOCK TABLES `setting` WRITE;
/*!40000 ALTER TABLE `setting` DISABLE KEYS */;
/*!40000 ALTER TABLE `setting` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `stack`
--

DROP TABLE IF EXISTS `stack`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `stack` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `external_id` varchar(128) DEFAULT NULL,
  `health_state` varchar(128) DEFAULT NULL,
  `folder` varchar(255) DEFAULT NULL,
  `parent_environment_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_environment_uuid` (`uuid`),
  KEY `fk_environment__account_id` (`account_id`),
  KEY `fk_environment_environment_id` (`parent_environment_id`),
  KEY `fk_stack__cluster_id` (`cluster_id`),
  KEY `fk_stack__creator_id` (`creator_id`),
  KEY `idx_environment_external_id` (`external_id`),
  KEY `idx_environment_name` (`name`),
  KEY `idx_environment_remove_time` (`remove_time`),
  KEY `idx_environment_removed` (`removed`),
  KEY `idx_environment_state` (`state`),
  CONSTRAINT `fk_environment__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_environment_environment_id` FOREIGN KEY (`parent_environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_stack__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_stack__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `stack`
--

LOCK TABLES `stack` WRITE;
/*!40000 ALTER TABLE `stack` DISABLE KEYS */;
/*!40000 ALTER TABLE `stack` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `storage_driver`
--

DROP TABLE IF EXISTS `storage_driver`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `storage_driver` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `service_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_storage_driver_uuid` (`uuid`),
  KEY `fk_storage_driver__cluster_id` (`cluster_id`),
  KEY `fk_storage_driver__service_id` (`service_id`),
  KEY `idx_storage_driver_name` (`name`),
  KEY `idx_storage_driver_remove_time` (`remove_time`),
  KEY `idx_storage_driver_removed` (`removed`),
  KEY `idx_storage_driver_state` (`state`),
  CONSTRAINT `fk_storage_driver__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_driver__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `storage_driver`
--

LOCK TABLES `storage_driver` WRITE;
/*!40000 ALTER TABLE `storage_driver` DISABLE KEYS */;
/*!40000 ALTER TABLE `storage_driver` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `storage_pool`
--

DROP TABLE IF EXISTS `storage_pool`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `storage_pool` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `physical_total_size_mb` bigint(20) DEFAULT NULL,
  `virtual_total_size_mb` bigint(20) DEFAULT NULL,
  `external` bit(1) NOT NULL DEFAULT b'0',
  `agent_id` bigint(20) DEFAULT NULL,
  `zone_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT NULL,
  `driver_name` varchar(255) DEFAULT NULL,
  `volume_access_mode` varchar(255) DEFAULT NULL,
  `storage_driver_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_storage_pool_uuid` (`uuid`),
  KEY `fk_storage_driver__id` (`storage_driver_id`),
  KEY `fk_storage_pool__agent_id` (`agent_id`),
  KEY `fk_storage_pool__cluster_id` (`cluster_id`),
  KEY `fk_storage_pool__zone_id` (`zone_id`),
  KEY `idx_storage_pool_name` (`name`),
  KEY `idx_storage_pool_remove_time` (`remove_time`),
  KEY `idx_storage_pool_removed` (`removed`),
  KEY `idx_storage_pool_state` (`state`),
  CONSTRAINT `fk_storage_driver__id` FOREIGN KEY (`storage_driver_id`) REFERENCES `storage_driver` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_pool__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_pool__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `storage_pool`
--

LOCK TABLES `storage_pool` WRITE;
/*!40000 ALTER TABLE `storage_pool` DISABLE KEYS */;
/*!40000 ALTER TABLE `storage_pool` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `storage_pool_host_map`
--

DROP TABLE IF EXISTS `storage_pool_host_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `storage_pool_host_map` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `storage_pool_id` bigint(20) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_storage_pool_host_map_uuid` (`uuid`),
  KEY `fk_storage_pool_host_map__host_id` (`host_id`),
  KEY `fk_storage_pool_host_map__storage_pool_id` (`storage_pool_id`),
  KEY `idx_storage_pool_host_map_name` (`name`),
  KEY `idx_storage_pool_host_map_remove_time` (`remove_time`),
  KEY `idx_storage_pool_host_map_removed` (`removed`),
  KEY `idx_storage_pool_host_map_state` (`state`),
  CONSTRAINT `fk_storage_pool_host_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_pool_host_map__storage_pool_id` FOREIGN KEY (`storage_pool_id`) REFERENCES `storage_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `storage_pool_host_map`
--

LOCK TABLES `storage_pool_host_map` WRITE;
/*!40000 ALTER TABLE `storage_pool_host_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `storage_pool_host_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `subnet`
--

DROP TABLE IF EXISTS `subnet`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `subnet` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `network_address` varchar(255) DEFAULT NULL,
  `cidr_size` int(11) DEFAULT NULL,
  `start_address` varchar(255) DEFAULT NULL,
  `end_address` varchar(255) DEFAULT NULL,
  `gateway` varchar(255) DEFAULT NULL,
  `network_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_subnet_uuid` (`uuid`),
  KEY `fk_subnet__cluster_id` (`cluster_id`),
  KEY `fk_subnet__network_id` (`network_id`),
  KEY `idx_subnet_name` (`name`),
  KEY `idx_subnet_remove_time` (`remove_time`),
  KEY `idx_subnet_removed` (`removed`),
  KEY `idx_subnet_state` (`state`),
  CONSTRAINT `fk_subnet__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_subnet__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `subnet`
--

LOCK TABLES `subnet` WRITE;
/*!40000 ALTER TABLE `subnet` DISABLE KEYS */;
/*!40000 ALTER TABLE `subnet` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ui_challenge`
--

DROP TABLE IF EXISTS `ui_challenge`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ui_challenge` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` varchar(255) DEFAULT NULL,
  `name` varchar(255) DEFAULT NULL,
  `email` varchar(255) NOT NULL,
  `token` varchar(255) NOT NULL,
  `data` mediumtext,
  `request` varchar(255) NOT NULL,
  `created` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `token` (`token`),
  KEY `created_token` (`created`,`token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ui_challenge`
--

LOCK TABLES `ui_challenge` WRITE;
/*!40000 ALTER TABLE `ui_challenge` DISABLE KEYS */;
/*!40000 ALTER TABLE `ui_challenge` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `user_preference`
--

DROP TABLE IF EXISTS `user_preference`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `user_preference` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `created` datetime DEFAULT NULL,
  `data` mediumtext,
  `value` mediumtext NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_preference_uuid` (`uuid`),
  KEY `fk_user_preference__account_id` (`account_id`),
  KEY `idx_user_preference_name` (`name`),
  CONSTRAINT `fk_user_preference__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `user_preference`
--

LOCK TABLES `user_preference` WRITE;
/*!40000 ALTER TABLE `user_preference` DISABLE KEYS */;
/*!40000 ALTER TABLE `user_preference` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `volume`
--

DROP TABLE IF EXISTS `volume`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `volume` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `physical_size_mb` bigint(20) DEFAULT NULL,
  `virtual_size_mb` bigint(20) DEFAULT NULL,
  `uri` varchar(255) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT NULL,
  `access_mode` varchar(255) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  `deployment_unit_id` bigint(20) DEFAULT NULL,
  `environment_id` bigint(20) DEFAULT NULL,
  `volume_template_id` bigint(20) DEFAULT NULL,
  `storage_driver_id` bigint(20) DEFAULT NULL,
  `size_mb` bigint(20) DEFAULT NULL,
  `storage_pool_id` bigint(20) DEFAULT NULL,
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_volume_uuid` (`uuid`),
  KEY `fk_volume__account_id` (`account_id`),
  KEY `fk_volume__cluster_id` (`cluster_id`),
  KEY `fk_volume__creator_id` (`creator_id`),
  KEY `fk_volume__deployment_unit_id` (`deployment_unit_id`),
  KEY `fk_volume__environment_id` (`environment_id`),
  KEY `fk_volume__host_id` (`host_id`),
  KEY `fk_volume__storage_driver_id` (`storage_driver_id`),
  KEY `fk_volume__storage_pool_id` (`storage_pool_id`),
  KEY `fk_volume__volume_template_id` (`volume_template_id`),
  KEY `idx_volume_external_id` (`external_id`),
  KEY `idx_volume_name` (`name`),
  KEY `idx_volume_remove_time` (`remove_time`),
  KEY `idx_volume_removed` (`removed`),
  KEY `idx_volume_state` (`state`),
  KEY `idx_volume_uri` (`uri`),
  CONSTRAINT `fk_volume__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__deployment_unit_id` FOREIGN KEY (`deployment_unit_id`) REFERENCES `deployment_unit` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__storage_driver_id` FOREIGN KEY (`storage_driver_id`) REFERENCES `storage_driver` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__storage_pool_id` FOREIGN KEY (`storage_pool_id`) REFERENCES `storage_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__volume_template_id` FOREIGN KEY (`volume_template_id`) REFERENCES `volume_template` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `volume`
--

LOCK TABLES `volume` WRITE;
/*!40000 ALTER TABLE `volume` DISABLE KEYS */;
/*!40000 ALTER TABLE `volume` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `volume_storage_pool_map`
--

DROP TABLE IF EXISTS `volume_storage_pool_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `volume_storage_pool_map` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `volume_id` bigint(20) DEFAULT NULL,
  `storage_pool_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_volume_storage_pool_map_uuid` (`uuid`),
  KEY `fk_volume_storage_pool_map__storage_pool_id` (`storage_pool_id`),
  KEY `fk_volume_storage_pool_map__volume_id` (`volume_id`),
  KEY `idx_volume_storage_pool_map_name` (`name`),
  KEY `idx_volume_storage_pool_map_remove_time` (`remove_time`),
  KEY `idx_volume_storage_pool_map_removed` (`removed`),
  KEY `idx_volume_storage_pool_map_state` (`state`),
  CONSTRAINT `fk_volume_storage_pool_map__storage_pool_id` FOREIGN KEY (`storage_pool_id`) REFERENCES `storage_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume_storage_pool_map__volume_id` FOREIGN KEY (`volume_id`) REFERENCES `volume` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `volume_storage_pool_map`
--

LOCK TABLES `volume_storage_pool_map` WRITE;
/*!40000 ALTER TABLE `volume_storage_pool_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `volume_storage_pool_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `volume_template`
--

DROP TABLE IF EXISTS `volume_template`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `volume_template` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` text,
  `driver` varchar(255) DEFAULT NULL,
  `environment_id` bigint(20) DEFAULT NULL,
  `external` bit(1) NOT NULL DEFAULT b'0',
  `per_container` bit(1) NOT NULL DEFAULT b'0',
  `cluster_id` bigint(20) NOT NULL,
  `creator_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_volume_template_uuid` (`uuid`),
  KEY `fk_volume_template__account_id` (`account_id`),
  KEY `fk_volume_template__cluster_id` (`cluster_id`),
  KEY `fk_volume_template__creator_id` (`creator_id`),
  KEY `fk_volume_template__environment_id` (`environment_id`),
  KEY `idx_volume_template_name` (`name`),
  KEY `idx_volume_template_remove_time` (`remove_time`),
  KEY `idx_volume_template_removed` (`removed`),
  KEY `idx_volume_template_state` (`state`),
  CONSTRAINT `fk_volume_template__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume_template__cluster_id` FOREIGN KEY (`cluster_id`) REFERENCES `cluster` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume_template__creator_id` FOREIGN KEY (`creator_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume_template__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `stack` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `volume_template`
--

LOCK TABLES `volume_template` WRITE;
/*!40000 ALTER TABLE `volume_template` DISABLE KEYS */;
/*!40000 ALTER TABLE `volume_template` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2017-09-14  0:44:24
