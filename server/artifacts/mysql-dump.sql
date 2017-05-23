-- MySQL dump 10.13  Distrib 5.5.53, for debian-linux-gnu (x86_64)
--
-- Host: localhost    Database: cattle
-- ------------------------------------------------------
-- Server version	5.5.53-0ubuntu0.14.04.1

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
  `LIQUIBASE` varchar(20) DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `DATABASECHANGELOG`
--

LOCK TABLES `DATABASECHANGELOG` WRITE;
/*!40000 ALTER TABLE `DATABASECHANGELOG` DISABLE KEYS */;
INSERT INTO `DATABASECHANGELOG` VALUES ('dump1','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',1,'EXECUTED','7:7804898bf856df0c94c17a1b6124775d','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',2,'EXECUTED','7:b15f282d3fc93ca53935cbd47997f265','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',3,'EXECUTED','7:f3466a18a83797c81a53c3f44422b61d','createTable','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',4,'EXECUTED','7:1374399fc3af8bd4098d25f05ca213bf','createTable','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',5,'EXECUTED','7:ff24046a894a2be1521667cda2f55e6b','createTable','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',6,'EXECUTED','7:33d2115150da7f9268b7e585c072d9a3','createTable','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',7,'EXECUTED','7:e6d928e1ed4b5e50f349cc29e8e9aaa0','createTable','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',8,'EXECUTED','7:4ae450d3862ff6481c9e2dddca552ae9','createTable','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',9,'EXECUTED','7:d444ae1b41d6b0ecf33d6fcadd480db5','createTable','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',10,'EXECUTED','7:03b36911af44b34fcbe84027f4a3052b','createTable','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',11,'EXECUTED','7:a7eca442e232d6d14324c38627ec743a','createTable','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',12,'EXECUTED','7:90f2fea19a5a8586933abfb6a37364eb','createTable','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',13,'EXECUTED','7:9a053b698f56edcb6e721af661530ad5','createTable','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',14,'EXECUTED','7:3bf47d59f4f7fb44657dedf3cea94aa3','createTable','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',15,'EXECUTED','7:a27991896abf35eb49eb874d3c420f96','createTable','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',16,'EXECUTED','7:9730d8ac4265b2495d3c27caf0346cb2','createTable','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',17,'EXECUTED','7:77126348d721c3058813ea1c85d7ea22','createTable','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',18,'EXECUTED','7:6eac595dbfe701763b64e6647355454e','createTable','',NULL,'3.1.0'),('dump19','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',19,'EXECUTED','7:173a2c487e08c0dfdc739b171a9b36cf','createTable','',NULL,'3.1.0'),('dump20','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',20,'EXECUTED','7:73a2f8a59d597bb7de5f12a3b63972c1','createTable','',NULL,'3.1.0'),('dump21','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',21,'EXECUTED','7:31aa5c7a50738f7a39153d2fecb70333','createTable','',NULL,'3.1.0'),('dump22','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',22,'EXECUTED','7:418b1781ad9cf268fb1e9a236bb57de3','createTable','',NULL,'3.1.0'),('dump23','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',23,'EXECUTED','7:bcea11c572a8f524086dc29f4ebe70b9','createTable','',NULL,'3.1.0'),('dump24','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',24,'EXECUTED','7:d830d0a491f5d867f707aa2cf7f7f430','createTable','',NULL,'3.1.0'),('dump25','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',25,'EXECUTED','7:cfc15d8869639fc8652cf79aa56f796a','createTable','',NULL,'3.1.0'),('dump26','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',26,'EXECUTED','7:5e1b8344cd14f1fe3da65a8d7ef1245a','createTable','',NULL,'3.1.0'),('dump27','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',27,'EXECUTED','7:f843206e53671f7b766c95335284df52','addForeignKeyConstraint','',NULL,'3.1.0'),('dump28','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',28,'EXECUTED','7:16c254219c0138cb61e146234d4a28b3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump29','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',29,'EXECUTED','7:092faf37ae9f52de8e526ea51d207fe5','addForeignKeyConstraint','',NULL,'3.1.0'),('dump30','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',30,'EXECUTED','7:8a5b4ef10e9ea77f8ca2826a18056aff','addForeignKeyConstraint','',NULL,'3.1.0'),('dump31','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',31,'EXECUTED','7:2cc87c697420ee6d3b2e1a4d25697c09','addForeignKeyConstraint','',NULL,'3.1.0'),('dump33','darren (generated)','db/core-001.xml','2015-09-15 23:21:58',32,'EXECUTED','7:4ff7d8e8b39818b93f15d7eafd741240','addForeignKeyConstraint','',NULL,'3.1.0'),('dump34','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',33,'EXECUTED','7:4226e9972a45e5a400af712b81881b73','addForeignKeyConstraint','',NULL,'3.1.0'),('dump35','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',34,'EXECUTED','7:6d9ab5b80234e154ee930d0d7eccdd0f','addForeignKeyConstraint','',NULL,'3.1.0'),('dump36','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',35,'EXECUTED','7:85b8199afc534c7d9c2f8bc18e02adb7','addForeignKeyConstraint','',NULL,'3.1.0'),('dump37','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',36,'EXECUTED','7:635cabddc60e4c4bc06e35e6f0df0297','addForeignKeyConstraint','',NULL,'3.1.0'),('dump38','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',37,'EXECUTED','7:8f52a0c5309b013a5fd4054ff30a8d6c','addForeignKeyConstraint','',NULL,'3.1.0'),('dump39','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',38,'EXECUTED','7:7f25d9412738b3248ef6a239822bc4f3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump40','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',39,'EXECUTED','7:2aa2ca4c63e95b877fa9674d0466d1cf','addForeignKeyConstraint','',NULL,'3.1.0'),('dump41','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',40,'EXECUTED','7:bb6b889e6302ea216310d6b7f5210ebd','addForeignKeyConstraint','',NULL,'3.1.0'),('dump42','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',41,'EXECUTED','7:e9f5a46795e093d71eb2d06b8d8b6e94','addForeignKeyConstraint','',NULL,'3.1.0'),('dump43','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',42,'EXECUTED','7:75cf9ae506c7c1cd6cedf056d0eda209','addForeignKeyConstraint','',NULL,'3.1.0'),('dump44','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',43,'EXECUTED','7:cac4f5da937764dd69f058d11eea4c0e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump45','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',44,'EXECUTED','7:63b8d3fe08950f115fe12d8259603c93','addForeignKeyConstraint','',NULL,'3.1.0'),('dump46','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',45,'EXECUTED','7:cad37d8734a5d7dfc51c7cd7f351762e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump47','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',46,'EXECUTED','7:883404d806143b19b5b11e57aae0ce36','addForeignKeyConstraint','',NULL,'3.1.0'),('dump48','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',47,'EXECUTED','7:cd424696031aa8eb0b287b4103bdd538','addForeignKeyConstraint','',NULL,'3.1.0'),('dump49','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',48,'EXECUTED','7:b3956cf6161e7fd54d0e6fe16f17a0da','addForeignKeyConstraint','',NULL,'3.1.0'),('dump50','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',49,'EXECUTED','7:d44b7a9d8e72d244ac91a81655685060','addForeignKeyConstraint','',NULL,'3.1.0'),('dump51','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',50,'EXECUTED','7:cb03ddc387db3cdf9b77e5bf4d930d5c','addForeignKeyConstraint','',NULL,'3.1.0'),('dump52','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',51,'EXECUTED','7:223cc172f33af606f4fe3ad09759a38d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump53','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',52,'EXECUTED','7:0097cde3f08d70e565b0ed0700e51b2a','addForeignKeyConstraint','',NULL,'3.1.0'),('dump54','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',53,'EXECUTED','7:d2c6114a561e2d1bc48e8c22e07a8d06','addForeignKeyConstraint','',NULL,'3.1.0'),('dump55','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',54,'EXECUTED','7:db21b39ec7d0578fa03bc9922e2a3e33','addForeignKeyConstraint','',NULL,'3.1.0'),('dump56','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',55,'EXECUTED','7:18d8c7a92a2b2f0eebd2b9cc44f1e965','addForeignKeyConstraint','',NULL,'3.1.0'),('dump57','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',56,'EXECUTED','7:9b121aea8e830c3d3af18183ce0c5620','addForeignKeyConstraint','',NULL,'3.1.0'),('dump58','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',57,'EXECUTED','7:59198dc847af4b914183366cbd9d047b','addForeignKeyConstraint','',NULL,'3.1.0'),('dump59','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',58,'EXECUTED','7:a1fae7dbcfca03d025d491cee1f3609c','addForeignKeyConstraint','',NULL,'3.1.0'),('dump60','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',59,'EXECUTED','7:d2331fad14fd961db0d217d0220c582b','addForeignKeyConstraint','',NULL,'3.1.0'),('dump61','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',60,'EXECUTED','7:c856ea11691bafece7b3abe9a5018909','addForeignKeyConstraint','',NULL,'3.1.0'),('dump62','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',61,'EXECUTED','7:de18b709b668aaae150b87645c563eca','addForeignKeyConstraint','',NULL,'3.1.0'),('dump63','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',62,'EXECUTED','7:006af553f652aeaf2aa68290aa9f1dbc','addForeignKeyConstraint','',NULL,'3.1.0'),('dump64','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',63,'EXECUTED','7:dc9ef476f62f1edb158dc2cbfc333ed6','addUniqueConstraint','',NULL,'3.1.0'),('dump65','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',64,'EXECUTED','7:d39fda2b41d9de0d7706a4200ec5fab7','addUniqueConstraint','',NULL,'3.1.0'),('dump66','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',65,'EXECUTED','7:e4d169fc5f37644ddcf492cf377215ac','addUniqueConstraint','',NULL,'3.1.0'),('dump67','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',66,'EXECUTED','7:f48fadc0834209596cadf74281a3eed9','addUniqueConstraint','',NULL,'3.1.0'),('dump68','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',67,'EXECUTED','7:4a7cd3e85430a01da60b211a7ad7a200','addUniqueConstraint','',NULL,'3.1.0'),('dump69','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',68,'EXECUTED','7:eb7886ddf499f0c91c279f5f3f2ac186','addUniqueConstraint','',NULL,'3.1.0'),('dump70','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',69,'EXECUTED','7:128b4ef004e988cfea93a31059294997','addUniqueConstraint','',NULL,'3.1.0'),('dump71','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',70,'EXECUTED','7:36de0bb235966ca3a54c80ba159d8b97','addUniqueConstraint','',NULL,'3.1.0'),('dump72','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',71,'EXECUTED','7:099b28b0434ece127e2867a092be4b55','addUniqueConstraint','',NULL,'3.1.0'),('dump73','darren (generated)','db/core-001.xml','2015-09-15 23:21:59',72,'EXECUTED','7:a136717a7da31bc2833332a728892303','addUniqueConstraint','',NULL,'3.1.0'),('dump74','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',73,'EXECUTED','7:053a0e19ad33e7188efe567665d50fb6','addUniqueConstraint','',NULL,'3.1.0'),('dump75','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',74,'EXECUTED','7:50aaf7c01c9615691c2c2985b3663dcd','addUniqueConstraint','',NULL,'3.1.0'),('dump76','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',75,'EXECUTED','7:3cf63f5658756e4bb1c6bf741c2bfbe2','addUniqueConstraint','',NULL,'3.1.0'),('dump77','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',76,'EXECUTED','7:2561d075d029ba946f3bcbde7484fbc8','addUniqueConstraint','',NULL,'3.1.0'),('dump78','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',77,'EXECUTED','7:2734de68ecce8d415892a4a2d1014509','addUniqueConstraint','',NULL,'3.1.0'),('dump79','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',78,'EXECUTED','7:aec74e6e897138431815bf9b83b33ba2','addUniqueConstraint','',NULL,'3.1.0'),('dump80','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',79,'EXECUTED','7:10432e1ef5ce9e780d0b84a486979311','addUniqueConstraint','',NULL,'3.1.0'),('dump81','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',80,'EXECUTED','7:1414d974977d180efe4f90c686f2cf2a','addUniqueConstraint','',NULL,'3.1.0'),('dump82','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',81,'EXECUTED','7:17dc1738baad40884cd5db55cea1e304','addUniqueConstraint','',NULL,'3.1.0'),('dump83','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',82,'EXECUTED','7:15a706509dc4bd17c6dbf110adcffbf6','addUniqueConstraint','',NULL,'3.1.0'),('dump84','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',83,'EXECUTED','7:3901c5e94e4b09110c21fe560468e428','addUniqueConstraint','',NULL,'3.1.0'),('dump85','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',84,'EXECUTED','7:4533a1ac9939ae985a3df0b474921317','addUniqueConstraint','',NULL,'3.1.0'),('dump86','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',85,'EXECUTED','7:e5733685acbc34894d46ef8f7646ebaf','addUniqueConstraint','',NULL,'3.1.0'),('dump87','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',86,'EXECUTED','7:7ebef6446989d8753e196e06c64dcf2a','createIndex','',NULL,'3.1.0'),('dump88','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',87,'EXECUTED','7:7f211b4bff5a60c3586e4dacf8471d69','createIndex','',NULL,'3.1.0'),('dump89','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',88,'EXECUTED','7:4235f7dadf568841987e5ff88b2a34df','createIndex','',NULL,'3.1.0'),('dump90','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',89,'EXECUTED','7:c8c3621e95f5ba073966d613ce878975','createIndex','',NULL,'3.1.0'),('dump91','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',90,'EXECUTED','7:d2b44cbc6a7ffd7bb40df8e9ed0c22eb','createIndex','',NULL,'3.1.0'),('dump92','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',91,'EXECUTED','7:602902b2073f8313dce25806205a0266','createIndex','',NULL,'3.1.0'),('dump93','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',92,'EXECUTED','7:dc6c8115e037f40b82a6840491d824a2','createIndex','',NULL,'3.1.0'),('dump94','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',93,'EXECUTED','7:5469eabe0933e3d574999aeca6a31390','createIndex','',NULL,'3.1.0'),('dump95','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',94,'EXECUTED','7:4bc29cf7565f01a325e9e22f4bb982c7','createIndex','',NULL,'3.1.0'),('dump96','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',95,'EXECUTED','7:251765fc75d3ce99464dc73bbeace618','createIndex','',NULL,'3.1.0'),('dump97','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',96,'EXECUTED','7:42903ec75b4af0cbd4e0fe4c22ce04c3','createIndex','',NULL,'3.1.0'),('dump98','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',97,'EXECUTED','7:5c1d95bb342421d5feb552120a1295fe','createIndex','',NULL,'3.1.0'),('dump99','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',98,'EXECUTED','7:2f6e73ec7f68d8ee9d79005138b967c6','createIndex','',NULL,'3.1.0'),('dump100','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',99,'EXECUTED','7:b532e2d0ea94dc11db68b9549c12efa1','createIndex','',NULL,'3.1.0'),('dump101','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',100,'EXECUTED','7:d3c049c295c654b9e0839f6c8bddd80c','createIndex','',NULL,'3.1.0'),('dump102','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',101,'EXECUTED','7:f03464e4aae83d1d379184ce9346a842','createIndex','',NULL,'3.1.0'),('dump103','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',102,'EXECUTED','7:45498f19b55aac4ba4da98a0eb310e4f','createIndex','',NULL,'3.1.0'),('dump104','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',103,'EXECUTED','7:63b8f2409744aa10ff49a5d330971c13','createIndex','',NULL,'3.1.0'),('dump105','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',104,'EXECUTED','7:6f5521a54d619702dfbc6ec39773e826','createIndex','',NULL,'3.1.0'),('dump106','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',105,'EXECUTED','7:fa8c27912f0935dcbb5bbace991ca0d5','createIndex','',NULL,'3.1.0'),('dump107','darren (generated)','db/core-001.xml','2015-09-15 23:22:00',106,'EXECUTED','7:88f7d448158ee36d80b87c9dccd869d5','createIndex','',NULL,'3.1.0'),('dump108','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',107,'EXECUTED','7:879cbc1c5af03ee6a71128c87a52d58e','createIndex','',NULL,'3.1.0'),('dump109','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',108,'EXECUTED','7:4650ba65b0fbfcf563c1d02ed767ae76','createIndex','',NULL,'3.1.0'),('dump110','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',109,'EXECUTED','7:53b6dd2a670711e55c64fa32adcf8c5d','createIndex','',NULL,'3.1.0'),('dump111','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',110,'EXECUTED','7:0cc7fc0d9787c101f94d028dbfdfe671','createIndex','',NULL,'3.1.0'),('dump112','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',111,'EXECUTED','7:7e57ce35d7ad8809c35010952c69138a','createIndex','',NULL,'3.1.0'),('dump113','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',112,'EXECUTED','7:bde8964dfb244a2cc492c351ad5b3646','createIndex','',NULL,'3.1.0'),('dump114','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',113,'EXECUTED','7:3a773ea7389aa3e69f4d42b7eaab45b2','createIndex','',NULL,'3.1.0'),('dump115','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',114,'EXECUTED','7:e49935cb09f45d66ef47695f4be03adb','createIndex','',NULL,'3.1.0'),('dump116','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',115,'EXECUTED','7:0f70001e0870008c99ee88d349d3605a','createIndex','',NULL,'3.1.0'),('dump117','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',116,'EXECUTED','7:3896212ef9d6ddc6b316bce32d286d25','createIndex','',NULL,'3.1.0'),('dump118','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',117,'EXECUTED','7:12f725998e55d971d46e04c7215577f8','createIndex','',NULL,'3.1.0'),('dump119','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',118,'EXECUTED','7:2d5f794ff33877fcca14c2dd49833f6b','createIndex','',NULL,'3.1.0'),('dump120','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',119,'EXECUTED','7:c21f6fcceaed54fec7e94b51f5c141ad','createIndex','',NULL,'3.1.0'),('dump121','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',120,'EXECUTED','7:31d7b3b105d3fbbab35ad93b28a5090f','createIndex','',NULL,'3.1.0'),('dump122','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',121,'EXECUTED','7:dbb9ec62e1ff0dd3de21a0c38d6a9055','createIndex','',NULL,'3.1.0'),('dump123','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',122,'EXECUTED','7:74e7656823d69240108f20cf7ca862f4','createIndex','',NULL,'3.1.0'),('dump124','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',123,'EXECUTED','7:c2015517d3b144cd9d170285a9eefed0','createIndex','',NULL,'3.1.0'),('dump125','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',124,'EXECUTED','7:c8c27a1d5e59d4f7f09ee10636cc741a','createIndex','',NULL,'3.1.0'),('dump126','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',125,'EXECUTED','7:7788b730c3b0b3467253e5b553ecb340','createIndex','',NULL,'3.1.0'),('dump127','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',126,'EXECUTED','7:8fb62ed17305303e374c399c305921ef','createIndex','',NULL,'3.1.0'),('dump128','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',127,'EXECUTED','7:e4eea4584d5ec5c6ab8c3401aba0c246','createIndex','',NULL,'3.1.0'),('dump129','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',128,'EXECUTED','7:8bc49e04b385b94640ce54d44d7f5aed','createIndex','',NULL,'3.1.0'),('dump130','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',129,'EXECUTED','7:296ca9e8cb12487d6cdc02d24894ed86','createIndex','',NULL,'3.1.0'),('dump131','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',130,'EXECUTED','7:a323e4cb4591798df369d800a9a06114','createIndex','',NULL,'3.1.0'),('dump132','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',131,'EXECUTED','7:a58954b7300f5025f238c31ec48d6070','createIndex','',NULL,'3.1.0'),('dump133','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',132,'EXECUTED','7:3c11c41cb58574ef2ca7db68d764bcc1','createIndex','',NULL,'3.1.0'),('dump134','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',133,'EXECUTED','7:6b4005c25a4505ecb09c337a23293d72','createIndex','',NULL,'3.1.0'),('dump135','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',134,'EXECUTED','7:784f952cca266edf09be8a3c8a6aa5e0','createIndex','',NULL,'3.1.0'),('dump136','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',135,'EXECUTED','7:1d66cb11c56f8e4299cf45435e36a735','createIndex','',NULL,'3.1.0'),('dump137','darren (generated)','db/core-001.xml','2015-09-15 23:22:01',136,'EXECUTED','7:de2c8ac23d324dab11bd6df18d8c112a','createIndex','',NULL,'3.1.0'),('dump138','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',137,'EXECUTED','7:d7d90e863257d28421d9e546fddb92d7','createIndex','',NULL,'3.1.0'),('dump139','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',138,'EXECUTED','7:92be6b3d128936e593f1288fe931dc1f','createIndex','',NULL,'3.1.0'),('dump140','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',139,'EXECUTED','7:bd76d39c5b4f958e5a067e03ab273988','createIndex','',NULL,'3.1.0'),('dump141','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',140,'EXECUTED','7:ddcf793b9521f46c75b298ae96cb191c','createIndex','',NULL,'3.1.0'),('dump142','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',141,'EXECUTED','7:77c417367d4446fee2a4eb047ce4797d','createIndex','',NULL,'3.1.0'),('dump143','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',142,'EXECUTED','7:f1f2beef37463db538593c808c2cc694','createIndex','',NULL,'3.1.0'),('dump144','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',143,'EXECUTED','7:7791200ddd958a6cd672228de331d37c','createIndex','',NULL,'3.1.0'),('dump145','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',144,'EXECUTED','7:9359ae811e9bbb9ed8f42470b35725ac','createIndex','',NULL,'3.1.0'),('dump146','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',145,'EXECUTED','7:a89bcc77421e46e0e44752bed5e76417','createIndex','',NULL,'3.1.0'),('dump147','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',146,'EXECUTED','7:ead2d45335f0b8adc05fa8d361d8f120','createIndex','',NULL,'3.1.0'),('dump148','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',147,'EXECUTED','7:18d4029d37e568f0d11e0f5b88e70738','createIndex','',NULL,'3.1.0'),('dump149','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',148,'EXECUTED','7:359a23c691262d085c3f05db0bae4017','createIndex','',NULL,'3.1.0'),('dump150','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',149,'EXECUTED','7:5bd182cf00ca276a2448593b6a962d72','createIndex','',NULL,'3.1.0'),('dump151','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',150,'EXECUTED','7:cac59da7b071b29eb6e03f8dba2e5840','createIndex','',NULL,'3.1.0'),('dump152','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',151,'EXECUTED','7:1de8a10a1d6cada7bc594081a6555856','createIndex','',NULL,'3.1.0'),('dump153','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',152,'EXECUTED','7:8707e9de2c73ae74269272f06a61e6e0','createIndex','',NULL,'3.1.0'),('dump154','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',153,'EXECUTED','7:359689d2ffbdfb9926f67e2573220c73','createIndex','',NULL,'3.1.0'),('dump155','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',154,'EXECUTED','7:b640857fedd3b5b7cd0d19ddf74a9613','createIndex','',NULL,'3.1.0'),('dump156','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',155,'EXECUTED','7:0e1e504d4c51ae203eb52f1a75d3c211','createIndex','',NULL,'3.1.0'),('dump157','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',156,'EXECUTED','7:a60cf8d1129602f3d4d3720f018481df','createIndex','',NULL,'3.1.0'),('dump158','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',157,'EXECUTED','7:a6dcdc82dbcbc5e156bac27aec2b46c8','createIndex','',NULL,'3.1.0'),('dump159','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',158,'EXECUTED','7:aec2d117078daec21bb5d2e886447493','createIndex','',NULL,'3.1.0'),('dump160','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',159,'EXECUTED','7:74c6c271f5479776bda9b3816363a08d','createIndex','',NULL,'3.1.0'),('dump161','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',160,'EXECUTED','7:5b239073066176b78afc445dc0bfe976','createIndex','',NULL,'3.1.0'),('dump162','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',161,'EXECUTED','7:5f1429576deb624cac429cd41169f944','createIndex','',NULL,'3.1.0'),('dump163','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',162,'EXECUTED','7:07493fc8ab0ca10fbd4efe4866b2209e','createIndex','',NULL,'3.1.0'),('dump164','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',163,'EXECUTED','7:03ce60c693fdb4c2ff6fcacb5f22a4e9','createIndex','',NULL,'3.1.0'),('dump165','darren (generated)','db/core-001.xml','2015-09-15 23:22:02',164,'EXECUTED','7:300f3f029c0ec3d6ae1c612a62c4ee8e','createIndex','',NULL,'3.1.0'),('config-item','darren (generated)','db/core-002.xml','2015-09-15 23:22:02',165,'EXECUTED','7:15d69a612d8c648c8d8436d390ebe7d8','addForeignKeyConstraint','',NULL,'3.1.0'),('sql1','darren','db/core-003.sql','2015-09-15 23:22:02',166,'EXECUTED','7:c05338a691f3799259443224f0966393','sql','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-004.xml','2015-09-15 23:22:02',167,'EXECUTED','7:83179a2f80e18d46291967995957a806','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-004.xml','2015-09-15 23:22:03',168,'EXECUTED','7:e25efea285290336405600022f1468dd','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',169,'EXECUTED','7:e7672efcd9c67ddd3212f64b79abfc46','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',170,'EXECUTED','7:522b800bfdb12f76499b4ab3f3eea7ee','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',171,'EXECUTED','7:71b62f42f4aca6b0b3ca24acbc42acaf','createTable','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',172,'EXECUTED','7:7cf9b144c46900b8470c426247f6fc5e','createTable','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',173,'EXECUTED','7:46ce1a62b15809b7e6299d3821a94335','createTable','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',174,'EXECUTED','7:05cad1d2e448ca79b1ffeee5ed082837','createTable','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',175,'EXECUTED','7:373e2c94238d10f6c4376e53b48f2c9f','createTable','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',176,'EXECUTED','7:a5d6f59942d138b6da3c230bcbf18e26','createTable','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',177,'EXECUTED','7:822c4e8f4b42bf72b927968faf986946','createTable','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',178,'EXECUTED','7:910023c7f372f6c6d15b291a703a3e9c','addForeignKeyConstraint','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',179,'EXECUTED','7:26b55de991b946b2cc59d143bb4335c2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',180,'EXECUTED','7:162345c24c9229eb9ed4e3d42cca0a38','addForeignKeyConstraint','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',181,'EXECUTED','7:7b3c2c7bfbe7b837b6f4371251006180','addForeignKeyConstraint','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',182,'EXECUTED','7:3c6112f161a3b1e6e85cd471015472cf','addForeignKeyConstraint','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',183,'EXECUTED','7:e9ea180333a69def022558fed2234f9e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',184,'EXECUTED','7:70e7dc9521220227d418fb85a1f5f753','addForeignKeyConstraint','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',185,'EXECUTED','7:3a4b13a77769e6b6ec6c60bde542b269','addForeignKeyConstraint','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',186,'EXECUTED','7:0106cc27d86570fd430fd12b8efed318','addForeignKeyConstraint','',NULL,'3.1.0'),('dump19','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',187,'EXECUTED','7:370fa49530c9dc748eb70c28fe53d42a','addForeignKeyConstraint','',NULL,'3.1.0'),('dump20','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',188,'EXECUTED','7:0789b285c2ca065630b312bb5681fba6','addForeignKeyConstraint','',NULL,'3.1.0'),('dump21','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',189,'EXECUTED','7:995108e92f27c94eee96daf9fa9cb5d2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump22','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',190,'EXECUTED','7:cbcaf73ed95f3ddc5168ed05db217025','addForeignKeyConstraint','',NULL,'3.1.0'),('dump23','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',191,'EXECUTED','7:56b9612f8e5d82e7117abcfae212cc47','addForeignKeyConstraint','',NULL,'3.1.0'),('dump24','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',192,'EXECUTED','7:12dd6faed06d85e569d9bf43be966634','addForeignKeyConstraint','',NULL,'3.1.0'),('dump25','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',193,'EXECUTED','7:50fcc672bd23ad855d6fda1ff1124999','addForeignKeyConstraint','',NULL,'3.1.0'),('dump26','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',194,'EXECUTED','7:0d27231d41a8707494086277e0cb1dc3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump27','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',195,'EXECUTED','7:7715aba840d55ecda65e072d4ee1d46a','addForeignKeyConstraint','',NULL,'3.1.0'),('dump28','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',196,'EXECUTED','7:5fd03eec06380251a6e896c437a2c886','addForeignKeyConstraint','',NULL,'3.1.0'),('dump29','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',197,'EXECUTED','7:39e1eb7bdcad44aad8700b138bcf08ec','addForeignKeyConstraint','',NULL,'3.1.0'),('dump30','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',198,'EXECUTED','7:b396137e16fd4dce129a4a7b6e1f812a','addUniqueConstraint','',NULL,'3.1.0'),('dump31','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',199,'EXECUTED','7:52effe69eccf25aa9fc71c8787bbab8e','addUniqueConstraint','',NULL,'3.1.0'),('dump32','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',200,'EXECUTED','7:b764b9e38b715a1e5e87d74253fbfe3c','addUniqueConstraint','',NULL,'3.1.0'),('dump33','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',201,'EXECUTED','7:4b47442183e3c96516bdedd571c161a0','addUniqueConstraint','',NULL,'3.1.0'),('dump34','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',202,'EXECUTED','7:7d6c1c46f7e0bb7eadeab777a6c2bb42','addUniqueConstraint','',NULL,'3.1.0'),('dump35','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',203,'EXECUTED','7:0155e06cca437e50fcfa843d3f7ccc9d','addUniqueConstraint','',NULL,'3.1.0'),('dump36','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',204,'EXECUTED','7:ce74fbf536358e31dcd76bf09030f1e8','addUniqueConstraint','',NULL,'3.1.0'),('dump37','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',205,'EXECUTED','7:5e50bca2434e89fa14ffebc1c3a60e96','addUniqueConstraint','',NULL,'3.1.0'),('dump38','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',206,'EXECUTED','7:ea4c8aa3f2ef0162a28c589ff61b4c17','addUniqueConstraint','',NULL,'3.1.0'),('dump39','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',207,'EXECUTED','7:6d70480552d04835ddabd5c679d0c5a4','addUniqueConstraint','',NULL,'3.1.0'),('dump40','darren (generated)','db/core-005.xml','2015-09-15 23:22:03',208,'EXECUTED','7:7edf6eac83832a0806c4131445127667','createIndex','',NULL,'3.1.0'),('dump41','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',209,'EXECUTED','7:7f28c0a4d675cbf80f7cef14fe971d64','createIndex','',NULL,'3.1.0'),('dump42','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',210,'EXECUTED','7:66d73c66f002cd19c7733780ec73e05f','createIndex','',NULL,'3.1.0'),('dump43','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',211,'EXECUTED','7:c34630fbd8b368a7a148b82b1dcbb106','createIndex','',NULL,'3.1.0'),('dump44','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',212,'EXECUTED','7:979f3f4ebd78480b03b42c36a9d53bc9','createIndex','',NULL,'3.1.0'),('dump45','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',213,'EXECUTED','7:871fd45b1b4589c4db5b25970406a02d','createIndex','',NULL,'3.1.0'),('dump46','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',214,'EXECUTED','7:5e876f3dea733e22052016f177ab8427','createIndex','',NULL,'3.1.0'),('dump47','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',215,'EXECUTED','7:0166ecb9fb67473d74a82512e8247ead','createIndex','',NULL,'3.1.0'),('dump48','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',216,'EXECUTED','7:ba40d9ef4f3fa9784a290d557cbeeb78','createIndex','',NULL,'3.1.0'),('dump49','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',217,'EXECUTED','7:7c197375dd022c73fd06c8afcc628fbe','createIndex','',NULL,'3.1.0'),('dump50','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',218,'EXECUTED','7:2841a1e45001074bdb5f94492d761349','createIndex','',NULL,'3.1.0'),('dump51','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',219,'EXECUTED','7:90d4987bfad94fb1100753be4dad166c','createIndex','',NULL,'3.1.0'),('dump52','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',220,'EXECUTED','7:d792ea30005128894897ddb16cbbbbbd','createIndex','',NULL,'3.1.0'),('dump53','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',221,'EXECUTED','7:5632c8131fc302aad19857aacc89f9f4','createIndex','',NULL,'3.1.0'),('dump54','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',222,'EXECUTED','7:38ccfe67b6fd2ed481dc9f1ec515b11f','createIndex','',NULL,'3.1.0'),('dump55','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',223,'EXECUTED','7:4140ce5da433ed77c93fccba6ebf1c85','createIndex','',NULL,'3.1.0'),('dump56','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',224,'EXECUTED','7:c58ab12b617ef77265fba9ee497c7a01','createIndex','',NULL,'3.1.0'),('dump57','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',225,'EXECUTED','7:b73c6f175efb544c8b42bd77edf60290','createIndex','',NULL,'3.1.0'),('dump58','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',226,'EXECUTED','7:d2e291b0f2f8f6f0f9ccd8ebcfbaa090','createIndex','',NULL,'3.1.0'),('dump59','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',227,'EXECUTED','7:63fb9ecb3e342ace2b40209789f918f8','createIndex','',NULL,'3.1.0'),('dump60','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',228,'EXECUTED','7:2db06f94630ecaafef4cf3c5b18a8972','createIndex','',NULL,'3.1.0'),('dump61','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',229,'EXECUTED','7:da358d45a11459c10440744c3f23eb9f','createIndex','',NULL,'3.1.0'),('dump62','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',230,'EXECUTED','7:a24bc627eb9119100ac65ba9d2cba35d','createIndex','',NULL,'3.1.0'),('dump63','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',231,'EXECUTED','7:c8c733bae08de72246657bc87d3ca359','createIndex','',NULL,'3.1.0'),('dump64','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',232,'EXECUTED','7:2fe276dcf12da67ce740e3681d3cecb5','createIndex','',NULL,'3.1.0'),('dump65','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',233,'EXECUTED','7:133f96410addd5523a4574b273e4b88b','createIndex','',NULL,'3.1.0'),('dump66','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',234,'EXECUTED','7:856f204b9435eea1ac265b0f6499fbc6','createIndex','',NULL,'3.1.0'),('dump67','darren (generated)','db/core-005.xml','2015-09-15 23:22:04',235,'EXECUTED','7:a428380845570b8dcf87a29b212f6fd3','createIndex','',NULL,'3.1.0'),('dump68','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',236,'EXECUTED','7:ad6a8cc1bb7b3daa167d77be87ca2adf','createIndex','',NULL,'3.1.0'),('dump69','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',237,'EXECUTED','7:0e4f1d30c839b46ee66d0094457f2f88','createIndex','',NULL,'3.1.0'),('dump70','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',238,'EXECUTED','7:3375fee3558253856d9c54e00407f0d6','createIndex','',NULL,'3.1.0'),('dump71','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',239,'EXECUTED','7:21774e7cf9dc69165f2d98caa9d317b5','createIndex','',NULL,'3.1.0'),('dump72','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',240,'EXECUTED','7:47fef5de26efa252870aff2ede61f577','createIndex','',NULL,'3.1.0'),('dump73','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',241,'EXECUTED','7:ebe81ce492b16208b4aea0da02b34a27','createIndex','',NULL,'3.1.0'),('dump74','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',242,'EXECUTED','7:f9115b477a689e500c1c0433115528dd','createIndex','',NULL,'3.1.0'),('dump75','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',243,'EXECUTED','7:7a0a6999d35df1d85baf3657993b3a39','createIndex','',NULL,'3.1.0'),('dump76','darren (generated)','db/core-005.xml','2015-09-15 23:22:05',244,'EXECUTED','7:cb767b84797dff283dc6dcf582c6bb01','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',245,'EXECUTED','7:c8755b4af01a4e260a7a4b3dd2d5a08d','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',246,'EXECUTED','7:d60a5d8c2db85e5bc332812f9a8b163f','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',247,'EXECUTED','7:d231b78eb7b569dcea9bc4f8ff3e30f6','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',248,'EXECUTED','7:77178c90f104f3af220eae67f1de5c6a','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',249,'EXECUTED','7:7b70c93ff319d2fe84dab857f211137d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',250,'EXECUTED','7:fcd64a4f8a0b88e77c9bd2649d014bb3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',251,'EXECUTED','7:7d592c0ac2607502f6d65af047039e84','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',252,'EXECUTED','7:1ce521b6b0bd606d27cbd0497fafd94e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',253,'EXECUTED','7:0e5117ddff33fe83e3b28afb31af81cc','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',254,'EXECUTED','7:9479a03985bf003abe09ded458e152ab','addUniqueConstraint','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',255,'EXECUTED','7:c4322121d5db28a8267d65725ae8dc04','addUniqueConstraint','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',256,'EXECUTED','7:dd8b074ff7e2556caba642dabe1501fc','createIndex','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',257,'EXECUTED','7:ec222b8970c4d93de2c3ef304ee7557b','createIndex','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',258,'EXECUTED','7:a8d1542da9bc108022b0643e1d97c189','createIndex','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',259,'EXECUTED','7:9cbbdd0ceaa949485ca278e28ac2f641','createIndex','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-006.xml','2015-09-15 23:22:05',260,'EXECUTED','7:8bd5a460b6cec1932ddcdc6a9be0b575','createIndex','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-006.xml','2015-09-15 23:22:06',261,'EXECUTED','7:4124ec1d63376d389658a57ed5b50582','createIndex','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-006.xml','2015-09-15 23:22:06',262,'EXECUTED','7:7573bd6d654ee549494f92963590e237','createIndex','',NULL,'3.1.0'),('dump19','darren (generated)','db/core-006.xml','2015-09-15 23:22:06',263,'EXECUTED','7:adea467c3aef015b7e60e2cb1e9fd63d','createIndex','',NULL,'3.1.0'),('dump20','darren (generated)','db/core-006.xml','2015-09-15 23:22:06',264,'EXECUTED','7:110d21be3220aaad1adc75c9410d64b2','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',265,'EXECUTED','7:20d8ecf89ce19bc19c3644d6cd055f8e','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',266,'EXECUTED','7:a28b30f2880b10afe3c93207f90c6ed7','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',267,'EXECUTED','7:9b5d12470e3c71a14c43a576c376e39a','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',268,'EXECUTED','7:a3699ca2b33ab45f91099f7cd94a314d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',269,'EXECUTED','7:50758a0dcb7f362829bb6c7ada93f473','addUniqueConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',270,'EXECUTED','7:d34a2671d167393237278d12833ee7ac','createIndex','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',271,'EXECUTED','7:f37f319cfeffb1026eb3518bd44db8c2','createIndex','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',272,'EXECUTED','7:47593533443af45d6efa98d13a06ff20','createIndex','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-007.xml','2015-09-15 23:22:06',273,'EXECUTED','7:07d088dea2087a9e2b934234a043b14a','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-008.xml','2015-09-15 23:22:06',274,'EXECUTED','7:7dad9721ab5ca94b78bb86b57dd7e9d5','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-008.xml','2015-09-15 23:22:06',275,'EXECUTED','7:a210a237999205263fab7a1942917da6','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-009.xml','2015-09-15 23:22:06',276,'EXECUTED','7:6a5554bca9b776a4053b791449e24296','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-009.xml','2015-09-15 23:22:06',277,'EXECUTED','7:80b4ade6dfd0d4d54231d80e16e4cb45','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-009.xml','2015-09-15 23:22:06',278,'EXECUTED','7:61b241c8872d6ea421c765190037165c','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-010.xml','2015-09-15 23:22:06',279,'EXECUTED','7:f3e5dbbf37c3dd66584dd8bfbd9a966b','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',280,'EXECUTED','7:6c9f951f8f8262b041b7b01eba9471a0','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',281,'EXECUTED','7:a1d153502c1036365915bebda0ebd83b','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',282,'EXECUTED','7:6401dceaabc8f5bfec5d1540a5c7cfab','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',283,'EXECUTED','7:512f5344c6d5a9b857b10d2f36c63f32','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',284,'EXECUTED','7:8faea7300f1e35aa688557baf7db738e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',285,'EXECUTED','7:84a561dd399904ede24e5a4aa8d70e6d','addUniqueConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',286,'EXECUTED','7:aa6f2c10466e5e5658146af1dfd19e6f','createIndex','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',287,'EXECUTED','7:127f54d267a76e620610d8d4229e6c9a','createIndex','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-011.xml','2015-09-15 23:22:06',288,'EXECUTED','7:e9bc5a6774039356c5bac9de7e03423c','createIndex','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-011.xml','2015-09-15 23:22:07',289,'EXECUTED','7:63b7a6f8b3505ea4b517673fa19b48a8','createIndex','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-011.xml','2015-09-15 23:22:07',290,'EXECUTED','7:8dfcf26f384ad6497346f603e7a871a7','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',291,'EXECUTED','7:8fb89bfaa9016de032f6b73c1b18b708','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',292,'EXECUTED','7:521da6ad8020ab2e59f9d93b3135d628','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',293,'EXECUTED','7:31d4419b8d38ca994fd86b66bea0882a','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',294,'EXECUTED','7:fad65b16b95c45bd92d38c5acdd3545d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',295,'EXECUTED','7:787e4306f76a2886ff3a8aff0f7b5b6d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',296,'EXECUTED','7:32c85c67b2edcf5389e3b2e1e8bfc9d0','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',297,'EXECUTED','7:cf8e41d630d9e0006186bea7c241739c','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',298,'EXECUTED','7:75f4cb2610bfa4de316eeacd748ae100','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',299,'EXECUTED','7:4416080f979a92ed32b7c0ca31886c6c','addUniqueConstraint','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',300,'EXECUTED','7:78e278c3dbc31f107d49f458240fe6c3','addUniqueConstraint','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',301,'EXECUTED','7:a1df7e82d56ec4527f8e64bde80ae09b','createIndex','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',302,'EXECUTED','7:c3701109e15ee5b571b1818502279999','createIndex','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',303,'EXECUTED','7:6854dba58bdf430199467c71151a95b0','createIndex','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',304,'EXECUTED','7:e74ced6b2183b34fb218a652bfc095b0','createIndex','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',305,'EXECUTED','7:445c72f5ff642991c0bee5b9130e980a','createIndex','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',306,'EXECUTED','7:2374a414aa496eba9f8a666395740959','createIndex','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',307,'EXECUTED','7:c5832cb2cbe416848c400fbd5aaa6bb6','createIndex','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-012.xml','2015-09-15 23:22:07',308,'EXECUTED','7:7995824cd3069fb1d76ede3c892a8133','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-013.xml','2015-09-15 23:22:07',309,'EXECUTED','7:51e8a25231dc3893dd1750d2122187c1','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-013.xml','2015-09-15 23:22:07',310,'EXECUTED','7:66eb6612356497196001fd8f3f7e799c','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-013.xml','2015-09-15 23:22:07',311,'EXECUTED','7:8139cdfc198c70346d2a355fd62c68dc','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-013.xml','2015-09-15 23:22:07',312,'EXECUTED','7:8f335afbf614eb567e2f86ed804708f3','addColumn','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-013.xml','2015-09-15 23:22:07',313,'EXECUTED','7:269fbdda5b4f62cb76e2ef162d6a9e78','addColumn','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-013.xml','2015-09-15 23:22:07',314,'EXECUTED','7:6d4cdc759e3865fad531c0273ab941fe','addUniqueConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-013.xml','2015-09-15 23:22:08',315,'EXECUTED','7:fc4be2b6f2a268cf2fe8b10c4594bc3e','createIndex','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-013.xml','2015-09-15 23:22:08',316,'EXECUTED','7:e0ae0ca5b05b36747615126a9d63bc1e','dropUniqueConstraint','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-013.xml','2015-09-15 23:22:08',317,'EXECUTED','7:16298b2cb5286eefcfa88e416a257651','dropIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',318,'EXECUTED','7:2bab47439792e776af643367d87a415c','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',319,'EXECUTED','7:479bc6997e79f8058b5fb5d8deec5eb0','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',320,'EXECUTED','7:92182af59046f7b68bc41820370cab33','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',321,'EXECUTED','7:4bd17f4099e4ecc8f2800c61024ae579','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',322,'EXECUTED','7:c92498236dbd50767d09af9f2909d399','addUniqueConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',323,'EXECUTED','7:06c1096772e91b58d9be501ffd5ee8a1','createIndex','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',324,'EXECUTED','7:bf085abfb02262f4d820610e1d863bb8','createIndex','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',325,'EXECUTED','7:311888aaed4a9e91d2aeffa3108a8899','createIndex','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-014.xml','2015-09-15 23:22:08',326,'EXECUTED','7:80dd8a24998b4b3df5fb6a9484592e4b','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-015.xml','2015-09-15 23:22:08',327,'EXECUTED','7:250772af0d72805d6039c355dc2e279c','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',328,'EXECUTED','7:4e3dae3ae6816b68f9da2de1228b3088','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',329,'EXECUTED','7:600e48bcf75a797e50183c3101b2495f','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',330,'EXECUTED','7:c6d77f1d32cbe6fc6dd18aa80ffa1c2b','addUniqueConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',331,'EXECUTED','7:87211ce1d05e99ea51e4e12d052fd2c4','createIndex','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',332,'EXECUTED','7:8cda9d4a0996f326404fc799b3d93c0e','createIndex','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',333,'EXECUTED','7:ceacd1e7d5cc786691999a5e88d04c8a','createIndex','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',334,'EXECUTED','7:941295e79a43fd7b3d80be6b3ed7fe5c','createIndex','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-016.xml','2015-09-15 23:22:08',335,'EXECUTED','7:46b1d2527bc447c51c4f9c1d07a348b2','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',336,'EXECUTED','7:515b8f95d0b2d7ab211a47f3cf156854','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',337,'EXECUTED','7:3f65b9830da11f2954acfe338bee0d01','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',338,'EXECUTED','7:c5f9bcb1128f1c81b27b217d645665a9','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',339,'EXECUTED','7:fd0367d2d735c1eb96e543cce0c24f34','addColumn','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',340,'EXECUTED','7:0217248f12a5bcbb6575f0bf5a84b194','addColumn','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',341,'EXECUTED','7:fa5a91c51e72d95c5ae16be4c69d7d07','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-017.xml','2015-09-15 23:22:08',342,'EXECUTED','7:f3850d30e4f0c36cc8399345262e7963','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',343,'EXECUTED','7:45552f1f1c6efc4bf969db77677d2f96','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',344,'EXECUTED','7:2d27ea0a27001f1e47b33ddb9a7b9685','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',345,'EXECUTED','7:9d23e86421a3b0db5d7c18b0169bd70d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',346,'EXECUTED','7:fa882ecaa1b30c0d429937227ce7ad01','addUniqueConstraint','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',347,'EXECUTED','7:2cdcda09fdcdb1a6b27014ac0969585b','addUniqueConstraint','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',348,'EXECUTED','7:1e92e9064b52b42b76b120f61ddc55df','createIndex','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',349,'EXECUTED','7:04c0b66c0fc4725088bb423e9c63cccf','createIndex','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',350,'EXECUTED','7:f2992da21dbd81ad2a5fc145b3572d00','createIndex','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',351,'EXECUTED','7:d5b2c6a144540fa4d006e130b921572a','createIndex','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',352,'EXECUTED','7:9857f0d8dbb665e78e85385bcbce8b99','createIndex','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',353,'EXECUTED','7:db58fe146c5c112d00033499441e773c','createIndex','',NULL,'3.1.0'),('dump19','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',354,'EXECUTED','7:3185c383864453ab20f5c3555ce55be2','createIndex','',NULL,'3.1.0'),('dump20','darren (generated)','db/core-017.xml','2015-09-15 23:22:09',355,'EXECUTED','7:ad8f8cfd742cafc81b79a6e5cde8a4bb','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',356,'EXECUTED','7:d7fdbc870ce409f066c4363822b64c46','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',357,'EXECUTED','7:389c7130ff1c7194b5d19ba4a4e4f993','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',358,'EXECUTED','7:f3afb49bc88eaf757a83161e788b5c3b','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',359,'EXECUTED','7:38126c0a1d5d0dc420b93a0d5f1b07ef','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',360,'EXECUTED','7:4d1cfb3fad6374e2000eb9b27ceef86d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',361,'EXECUTED','7:d15f801ccfe1751e1fb4e5b92dc37149','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',362,'EXECUTED','7:dc44d921e9138fd824c9300ec58b44a8','addUniqueConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',363,'EXECUTED','7:c58a5925df5bc1a79162184d96bda443','createIndex','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',364,'EXECUTED','7:fc34f6ae99d4006eff0028a500146704','createIndex','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',365,'EXECUTED','7:8560b95861cd6048ed97ff61517405f9','createIndex','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-018.xml','2015-09-15 23:22:09',366,'EXECUTED','7:d8cb1fd2a6953f43fa75af0082c07e40','createIndex','',NULL,'3.1.0'),('1417022638612-1','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:09',367,'EXECUTED','7:2e518b043ae7b1bad9fec2e35924e531','createTable','',NULL,'3.1.0'),('1417022638612-2','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:09',368,'EXECUTED','7:f9e0ece9b4f94045b47a4895ad7e1c36','addColumn','',NULL,'3.1.0'),('1417022638612-3','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:09',369,'EXECUTED','7:ab2cefa4916f98c2a238e4af647edd12','addUniqueConstraint','',NULL,'3.1.0'),('1417022638612-4','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',370,'EXECUTED','7:aed285f44ff6c66be6786dcfb35662ba','addForeignKeyConstraint','',NULL,'3.1.0'),('1417022638612-5','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',371,'EXECUTED','7:cbf7eedc27f3782ef0a0fbcb6121b748','addForeignKeyConstraint','',NULL,'3.1.0'),('1417022638612-6','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',372,'EXECUTED','7:5189c7af91eb55fcaf66b1c431c26967','addForeignKeyConstraint','',NULL,'3.1.0'),('1417022638612-7','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',373,'EXECUTED','7:a2bee99c66a43346254df77117761261','createIndex','',NULL,'3.1.0'),('1417022638612-8','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',374,'EXECUTED','7:da609ff59d7bd12e32693b1758be7723','createIndex','',NULL,'3.1.0'),('1417022638612-9','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',375,'EXECUTED','7:ac4b0a9e94721555d778f3cfd97328ea','createIndex','',NULL,'3.1.0'),('1417022638612-10','cjellick (generated)','db/core-019.xml','2015-09-15 23:22:10',376,'EXECUTED','7:e584158fc36a6ce459cc924a37ba03e3','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-020.xml','2015-09-15 23:22:10',377,'EXECUTED','7:aad177077287c38ed45dfd35a61f1e39','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-020.xml','2015-09-15 23:22:10',378,'EXECUTED','7:4d5e2e5b02d7b0cdf99657677f05d1a3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',379,'EXECUTED','7:04475221a8dc77d6916170c511b5b90b','createTable','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',380,'EXECUTED','7:8db3b3642fe1863261f399e32e1a0bd2','createTable','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',381,'EXECUTED','7:fc2444a4202609e1877f3927cdba49c6','createTable','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',382,'EXECUTED','7:0ea32b3e56457ea108553ccac06fbe21','createTable','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',383,'EXECUTED','7:5c4885ca140817fd040b5f0f6636d25e','createTable','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',384,'EXECUTED','7:7f7d24f61758ac6f5f5bb530505bb712','createTable','',NULL,'3.1.0'),('dump7','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',385,'EXECUTED','7:932a200c88b2dc6a073caa03f3c534ce','createTable','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',386,'EXECUTED','7:981daae79b6c2675be1633e202be6288','addUniqueConstraint','',NULL,'3.1.0'),('dump9','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',387,'EXECUTED','7:40f78257bd0ca687773b83b7036318b7','addUniqueConstraint','',NULL,'3.1.0'),('dump10','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',388,'EXECUTED','7:9989cb535120351a17eb7ceb277b053c','addUniqueConstraint','',NULL,'3.1.0'),('dump11','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',389,'EXECUTED','7:cc9455a0af825573fe518e2f184454d7','addUniqueConstraint','',NULL,'3.1.0'),('dump12','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',390,'EXECUTED','7:67a4c50465290b48d3b90f4cca9c2c61','addUniqueConstraint','',NULL,'3.1.0'),('dump13','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',391,'EXECUTED','7:b3e9120dab89a32dac6d48be44ef63ad','addUniqueConstraint','',NULL,'3.1.0'),('dump14','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',392,'EXECUTED','7:cfa1d494920e62f5de70e4678b9c28a4','addUniqueConstraint','',NULL,'3.1.0'),('dump15','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',393,'EXECUTED','7:243d7edd6a1b2cdf99d9368221bc48ef','addForeignKeyConstraint','',NULL,'3.1.0'),('dump16','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',394,'EXECUTED','7:4c4ab3943cad270b71af61a0b6a15873','addForeignKeyConstraint','',NULL,'3.1.0'),('dump17','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',395,'EXECUTED','7:903a9a26180c97bd97eca75e4f23f59e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump18','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',396,'EXECUTED','7:1218948dba5b2c1c35dfc9e3cfa69863','addForeignKeyConstraint','',NULL,'3.1.0'),('dump20','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',397,'EXECUTED','7:14103a7a1edf1d5192a8388a3f26ec35','addForeignKeyConstraint','',NULL,'3.1.0'),('dump21','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',398,'EXECUTED','7:41c78e95b389157caf1a2441fe613537','addForeignKeyConstraint','',NULL,'3.1.0'),('dump23','alena (generated)','db/core-021.xml','2015-09-15 23:22:10',399,'EXECUTED','7:165a7d207630bd8bc59c28a7656cd40b','addForeignKeyConstraint','',NULL,'3.1.0'),('dump24','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',400,'EXECUTED','7:f64491ede253627219bd7f046c7301fd','addForeignKeyConstraint','',NULL,'3.1.0'),('dump25','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',401,'EXECUTED','7:d479246560be0d5830cdee8ab58ca9bf','addForeignKeyConstraint','',NULL,'3.1.0'),('dump27','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',402,'EXECUTED','7:4de22ff3acdb68d2c8a1dd1c809c33b6','addForeignKeyConstraint','',NULL,'3.1.0'),('dump28','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',403,'EXECUTED','7:aa09507bedcd4e2c21aad63d548c8c02','addForeignKeyConstraint','',NULL,'3.1.0'),('dump29','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',404,'EXECUTED','7:d843cccd40e3b27f327e5e86052cde7a','createIndex','',NULL,'3.1.0'),('dump30','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',405,'EXECUTED','7:ee29dd215b86c58860fa6f12ec2247ef','createIndex','',NULL,'3.1.0'),('dump31','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',406,'EXECUTED','7:585a29c5e9a861031322e2a0e4a2e36f','createIndex','',NULL,'3.1.0'),('dump32','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',407,'EXECUTED','7:4ff532e60fe364490e7acdec7376ab14','createIndex','',NULL,'3.1.0'),('dump33','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',408,'EXECUTED','7:4b309b8e7d8b8d7c9628ef86fccdbeb4','createIndex','',NULL,'3.1.0'),('dump34','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',409,'EXECUTED','7:84e753df56a0ce71d2b68b0dab9f4037','createIndex','',NULL,'3.1.0'),('dump35','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',410,'EXECUTED','7:937d2135e5f6a7f13dd8844ad2d05021','createIndex','',NULL,'3.1.0'),('dump36','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',411,'EXECUTED','7:2de60531b74045578bf9ca7f93c1f304','createIndex','',NULL,'3.1.0'),('dump37','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',412,'EXECUTED','7:d99e642b3382dc724f89b12528cee3ef','createIndex','',NULL,'3.1.0'),('dump38','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',413,'EXECUTED','7:a5310f0c0ab50a35d274d4ce934b6527','createIndex','',NULL,'3.1.0'),('dump39','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',414,'EXECUTED','7:a375a80a96aeaa0dabf214ed01102fb4','createIndex','',NULL,'3.1.0'),('dump40','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',415,'EXECUTED','7:34711ee2d97c0a23dfcb4ee54fd0760e','createIndex','',NULL,'3.1.0'),('dump41','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',416,'EXECUTED','7:aa40f1fec209d1a28ccc3fac4054319e','createIndex','',NULL,'3.1.0'),('dump42','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',417,'EXECUTED','7:28dc1ef0da1f1a3bf029a4ab15fc27c8','createIndex','',NULL,'3.1.0'),('dump43','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',418,'EXECUTED','7:a11c91411ffe413c193e983049a7365e','createIndex','',NULL,'3.1.0'),('dump44','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',419,'EXECUTED','7:8327652db008491db5bfb2d6e8301b21','createIndex','',NULL,'3.1.0'),('dump45','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',420,'EXECUTED','7:b7110ecafc6264c55e785c48e4326ccc','createIndex','',NULL,'3.1.0'),('dump46','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',421,'EXECUTED','7:71c8e72d3f821cdc324845f32f15169f','createIndex','',NULL,'3.1.0'),('dump47','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',422,'EXECUTED','7:d54270debb2828a03c67b9ade116c2f2','createIndex','',NULL,'3.1.0'),('dump48','alena (generated)','db/core-021.xml','2015-09-15 23:22:11',423,'EXECUTED','7:3e57b06c93da69abaed606a50bf3368f','createIndex','',NULL,'3.1.0'),('dump49','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',424,'EXECUTED','7:017cf96e8e01999b56f0ce91ada4dbf7','createIndex','',NULL,'3.1.0'),('dump50','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',425,'EXECUTED','7:9299512e5a12d903db0084aa1eb95a0e','createIndex','',NULL,'3.1.0'),('dump51','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',426,'EXECUTED','7:3487749775fdb9bdeada134dcaf7a295','createIndex','',NULL,'3.1.0'),('dump52','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',427,'EXECUTED','7:f927f8611f86ec5d52788b899cfcae56','createIndex','',NULL,'3.1.0'),('dump53','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',428,'EXECUTED','7:2f09c29fc5bf8831252579c7f95927e0','createIndex','',NULL,'3.1.0'),('dump54','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',429,'EXECUTED','7:7b3ced244eb0c65a9c334d547b7efe95','createIndex','',NULL,'3.1.0'),('dump55','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',430,'EXECUTED','7:7a819e5e7d69a87493af3ec3d889640f','createIndex','',NULL,'3.1.0'),('dump56','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',431,'EXECUTED','7:b700a4374256027c7bb37c2a187f3153','createIndex','',NULL,'3.1.0'),('dump57','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',432,'EXECUTED','7:46dce0acebc85d3939acb0206f1967cd','addColumn','',NULL,'3.1.0'),('dump58','alena (generated)','db/core-021.xml','2015-09-15 23:22:12',433,'EXECUTED','7:94099e3362fb97e2dac913ba96eec6c4','addForeignKeyConstraint','',NULL,'3.1.0'),('1419858336104-1','cjellick (generated)','db/core-022.xml','2015-09-15 23:22:12',434,'EXECUTED','7:d4f6157a561f353c4249c94fe0df0cca','addColumn','',NULL,'3.1.0'),('dump1','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',435,'EXECUTED','7:74bba6d893d4bc7697b2d42e58727102','createTable','',NULL,'3.1.0'),('dump2','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',436,'EXECUTED','7:c32999ff107c224b580c18e210d53692','createTable','',NULL,'3.1.0'),('dump3','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',437,'EXECUTED','7:a230bbcd3681a36ce02e9377ae5f7274','addUniqueConstraint','',NULL,'3.1.0'),('dump4','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',438,'EXECUTED','7:2289db6906e238ba60e9b6483b3462e1','addUniqueConstraint','',NULL,'3.1.0'),('dump5','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',439,'EXECUTED','7:25c4498eabba600f3022a6e36cebddc5','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',440,'EXECUTED','7:552cca8ad31ceed2a411333e6f914e45','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',441,'EXECUTED','7:3855e681708cb3c1463e7d25856a344b','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',442,'EXECUTED','7:e1e08ad1476b9014d28926486b786833','createIndex','',NULL,'3.1.0'),('dump9','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',443,'EXECUTED','7:d1455b154c96ea9dd78ff09b412b2443','createIndex','',NULL,'3.1.0'),('dump10','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',444,'EXECUTED','7:705795e10d19c5b329aad57affad4613','createIndex','',NULL,'3.1.0'),('dump11','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',445,'EXECUTED','7:f3216440bf9c06f702ec84b2a09e9f24','createIndex','',NULL,'3.1.0'),('dump12','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',446,'EXECUTED','7:acd7a3e1e8e753a5b99226a01027a4f9','createIndex','',NULL,'3.1.0'),('dump13','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',447,'EXECUTED','7:d8584da370fa4aa12d03fd0af5f6964f','createIndex','',NULL,'3.1.0'),('dump14','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:12',448,'EXECUTED','7:8f76135afeb02dea9d96b07cdaa789d7','createIndex','',NULL,'3.1.0'),('dump15','sonchang (generated)','db/core-023.xml','2015-09-15 23:22:13',449,'EXECUTED','7:e292a665812a37354922735de940f3e4','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-024.xml','2015-09-15 23:22:13',450,'EXECUTED','7:cb450fc5e386228ba094f46c35847fe4','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-025.xml','2015-09-15 23:22:13',451,'EXECUTED','7:5caf7174e773ee53bfea9465e0e556e0','modifyDataType','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-025.xml','2015-09-15 23:22:13',452,'EXECUTED','7:854f05f85064717829752c4d789f3e59','modifyDataType','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-026.xml','2015-09-15 23:22:13',453,'EXECUTED','7:11b0bc59845f4e4c19e9010c129386f2','addColumn','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-026.xml','2015-09-15 23:22:13',454,'EXECUTED','7:3dafa880e0cdd91fe1f7e80d5de0957b','addColumn','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-026.xml','2015-09-15 23:22:13',455,'EXECUTED','7:7305343113201d6208c4d2abef8161c2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-026.xml','2015-09-15 23:22:13',456,'EXECUTED','7:01d79871d444a4bfdd4e26532f11ff97','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-027.xml','2015-09-15 23:22:13',457,'EXECUTED','7:eb95c9e6277bab33474146b3561618ee','addColumn','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-027.xml','2015-09-15 23:22:13',458,'EXECUTED','7:ecff6c8233b0206c362aafe0b70b8d97','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-028.xml','2015-09-15 23:22:13',459,'EXECUTED','7:472a28e46dcc754b9db5e892a84be267','addColumn','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-028.xml','2015-09-15 23:22:13',460,'EXECUTED','7:9851adc933f060375a662a93cad85d63','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-028.xml','2015-09-15 23:22:13',461,'EXECUTED','7:855cdd406761ec27447fb06d8be9d9ae','dropForeignKeyConstraint','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-028.xml','2015-09-15 23:22:13',462,'EXECUTED','7:9b3eade8e71f4bbfe49365d72c4893fa','dropColumn','',NULL,'3.1.0'),('dump1','willchan (generated)','db/core-029.xml','2015-09-15 23:22:13',463,'EXECUTED','7:4decdb85150b780c6b0a2d4ff3cb8b5a','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',464,'EXECUTED','7:940c63d102f545ec38c895a5939143f4','createTable','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',465,'EXECUTED','7:f9cdd5265f06d4786a2b948d299279e5','createTable','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',466,'EXECUTED','7:dc7a9e0c18580f57f3a4ba1afded7514','createTable','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',467,'EXECUTED','7:a5127561cb0c7554e23f6bcc0a924df7','createTable','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',468,'EXECUTED','7:955c758b639c58663230d865029166bf','addUniqueConstraint','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',469,'EXECUTED','7:f226a543b508573fc7df6001f7b1f379','addUniqueConstraint','',NULL,'3.1.0'),('dump9','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',470,'EXECUTED','7:f0cd6c848242ddaa8e76a8238252f15f','addUniqueConstraint','',NULL,'3.1.0'),('dump10','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',471,'EXECUTED','7:163f088d946d43e56314b746523cf6bc','addUniqueConstraint','',NULL,'3.1.0'),('dump11','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',472,'EXECUTED','7:9464cbd8921cd3e3ee9978d4ab0f5677','addForeignKeyConstraint','',NULL,'3.1.0'),('dump14','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',473,'EXECUTED','7:380b97b22c75ecd7a94a372a07a219ae','addForeignKeyConstraint','',NULL,'3.1.0'),('dump15','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',474,'EXECUTED','7:87088a918f3120137cb5aab62ccbec37','addForeignKeyConstraint','',NULL,'3.1.0'),('dump16','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',475,'EXECUTED','7:d623f6855a3861aeb9e2ba2e6aa862f7','addForeignKeyConstraint','',NULL,'3.1.0'),('dump17','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',476,'EXECUTED','7:416d8d5ac1a70629d63eb12ca221522a','addForeignKeyConstraint','',NULL,'3.1.0'),('dump18','alena (generated)','db/core-030.xml','2015-09-15 23:22:13',477,'EXECUTED','7:91e5d90ce37695500da9865da1518189','addForeignKeyConstraint','',NULL,'3.1.0'),('dump19','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',478,'EXECUTED','7:842bfdb2b35aa552315a1ca751959933','addForeignKeyConstraint','',NULL,'3.1.0'),('dump20','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',479,'EXECUTED','7:c97c3e08fdadab7f826ad51e01d6278f','createIndex','',NULL,'3.1.0'),('dump21','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',480,'EXECUTED','7:4c4e185b0234cc02bc1d02b892981ffb','createIndex','',NULL,'3.1.0'),('dump22','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',481,'EXECUTED','7:03b88f8537d5416fadab0638151eb547','createIndex','',NULL,'3.1.0'),('dump23','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',482,'EXECUTED','7:a3efdf2af0f651a6ebb6eaf5e8bd4cee','createIndex','',NULL,'3.1.0'),('dump28','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',483,'EXECUTED','7:5a27e20d11f1713d1007d350eeeba2d1','createIndex','',NULL,'3.1.0'),('dump29','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',484,'EXECUTED','7:598c39195d181fc9af442b360c0a878e','createIndex','',NULL,'3.1.0'),('dump30','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',485,'EXECUTED','7:29882a8cf2b6758e9ee4320eb1c666d0','createIndex','',NULL,'3.1.0'),('dump31','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',486,'EXECUTED','7:59f5a5e2bf62c0a2512cdef547ee72e8','createIndex','',NULL,'3.1.0'),('dump32','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',487,'EXECUTED','7:2a2085e1cfca9594e6a506fd73be6f56','createIndex','',NULL,'3.1.0'),('dump33','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',488,'EXECUTED','7:568aa35ae8b471f6ead5af7b74cc5a73','createIndex','',NULL,'3.1.0'),('dump34','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',489,'EXECUTED','7:589ec4ab33904f27d8eca5198ed1fcf7','createIndex','',NULL,'3.1.0'),('dump35','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',490,'EXECUTED','7:77bbfee828bd1cb1b0cc9ed79d2c36a0','createIndex','',NULL,'3.1.0'),('dump36','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',491,'EXECUTED','7:86412915261ef948b2bae082de6e70d1','createIndex','',NULL,'3.1.0'),('dump37','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',492,'EXECUTED','7:1a16361fc4e1798da0162f65365e4971','createIndex','',NULL,'3.1.0'),('dump38','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',493,'EXECUTED','7:6eb30214acec8a9498dc277d6d625afc','createIndex','',NULL,'3.1.0'),('dump39','alena (generated)','db/core-030.xml','2015-09-15 23:22:14',494,'EXECUTED','7:ec61d6b0701c26c6f478506253275cd1','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-031.xml','2015-09-15 23:22:14',495,'EXECUTED','7:04ac505bb7a65ce768905c6b745b07fd','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-031.xml','2015-09-15 23:22:14',496,'EXECUTED','7:bc342960557e1528ddcc3046062a81c9','addColumn','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-031.xml','2015-09-15 23:22:14',497,'EXECUTED','7:617c7522563f206ee7a3c7dfc25e9a8e','addColumn','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-031.xml','2015-09-15 23:22:14',498,'EXECUTED','7:8113af0c76ffaff6770e331b3a66c382','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-031.xml','2015-09-15 23:22:14',499,'EXECUTED','7:2e2121bc908cadd2ce9692c458036265','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-031.xml','2015-09-15 23:22:14',500,'EXECUTED','7:55fc05ac450f252835204ddd93b14dd1','addForeignKeyConstraint','',NULL,'3.1.0'),('1428469259608-1','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:14',501,'EXECUTED','7:7cbc7707dd8de3af3d0a44c622273318','createTable','',NULL,'3.1.0'),('1428469259608-2','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',502,'EXECUTED','7:5591924c420f7ea7c2d44a6b260bed20','addColumn','',NULL,'3.1.0'),('1428469259608-3','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',503,'EXECUTED','7:561cada2af0e919ed41a1da4ecebeda9','addColumn','',NULL,'3.1.0'),('1428469259608-4','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',504,'EXECUTED','7:b9c2f3647c9a4305a100ef4774ffb21b','addForeignKeyConstraint','',NULL,'3.1.0'),('1428469259608-5','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',505,'EXECUTED','7:35c8ca10a71822bba2d25374f1fbcb2a','addForeignKeyConstraint','',NULL,'3.1.0'),('1428469259608-6','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',506,'EXECUTED','7:aea6be232a2bd2eff4602f981b836455','createIndex','',NULL,'3.1.0'),('1428469259608-7','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',507,'EXECUTED','7:9e5d65984ea7a995659ae127e44c0df2','createIndex','',NULL,'3.1.0'),('1428469259608-8','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',508,'EXECUTED','7:9c2245d8d8f2bf5ee388261cb5e6e458','createIndex','',NULL,'3.1.0'),('1428469259608-9','cjellick (generated)','db/core-032.xml','2015-09-15 23:22:15',509,'EXECUTED','7:f7611a385bc2bfbf02ffaf13de0fd74e','createIndex','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',510,'EXECUTED','7:5ce64c2b780f8d757bf603e19d5cf991','createTable','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',511,'EXECUTED','7:2b27eba07279d754e598de194e83c44b','addColumn','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',512,'EXECUTED','7:3c7cae64fb5c26a103e2a4c271b17c02','addUniqueConstraint','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',513,'EXECUTED','7:20c9fac3f8c40535f8981434ca605489','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',514,'EXECUTED','7:1d057b6332fda7c85edb7c5f8aba178e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',515,'EXECUTED','7:949fa326f5e7ff27ac9f5ca0e97a39cf','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',516,'EXECUTED','7:c171b5402eb0912f04883349e185afb1','createIndex','',NULL,'3.1.0'),('dump8','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',517,'EXECUTED','7:5d7c3e2979350de2f5ec3e694a464907','createIndex','',NULL,'3.1.0'),('dump9','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',518,'EXECUTED','7:d7554ea6044d08e34fda839fa8c89a7f','createIndex','',NULL,'3.1.0'),('dump10','wizardofmath (generated)','db/core-033.xml','2015-09-15 23:22:15',519,'EXECUTED','7:5f19af3137ebb6ccc9309b01ee94d868','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-034.xml','2015-09-15 23:22:15',520,'EXECUTED','7:76c5e34663b3dd26890d840eac05f97a','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-034.xml','2015-09-15 23:22:15',521,'EXECUTED','7:593d53f5e8e64be921cce9b0f6224147','addColumn','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-034.xml','2015-09-15 23:22:15',522,'EXECUTED','7:2144f8a1891ca74d858b81ad693e9a15','addColumn','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-034.xml','2015-09-15 23:22:16',523,'EXECUTED','7:61c977ac7eee5704fe58498a66919ca5','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-034.xml','2015-09-15 23:22:16',524,'EXECUTED','7:bf12010f9ba3322508eef7a7da6cdec2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-034.xml','2015-09-15 23:22:16',525,'EXECUTED','7:3a740723251f252059fda3a193b92be0','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',526,'EXECUTED','7:13b0ee1960292de638731d505ffba85c','createTable','',NULL,'3.1.0'),('dump2','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',527,'EXECUTED','7:265f701d9d0e655e8e25c7a9cd537bb1','createTable','',NULL,'3.1.0'),('dump3','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',528,'EXECUTED','7:0f9276800aa17fc22cee8ea09dcae1f4','addUniqueConstraint','',NULL,'3.1.0'),('dump4','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',529,'EXECUTED','7:efbd68355012e055e828a7ac418cdedc','addUniqueConstraint','',NULL,'3.1.0'),('dump5','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',530,'EXECUTED','7:7ac5d70c49a75708b84450005626c489','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',531,'EXECUTED','7:3ce02a0791497883d005e5335e117c9b','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',532,'EXECUTED','7:52aa4132d4753bec956984ee2666bba0','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',533,'EXECUTED','7:aa151034a84e76a9c99c0fc0516cd659','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',534,'EXECUTED','7:f8477cd1d2b0ea86ffcb52bbfe224ab9','createIndex','',NULL,'3.1.0'),('dump10','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',535,'EXECUTED','7:fa5e82ac51bb39b34cd12e467893ba3b','createIndex','',NULL,'3.1.0'),('dump11','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',536,'EXECUTED','7:81fc66edcd24829bb1f5b9b9e4d95ce2','createIndex','',NULL,'3.1.0'),('dump12','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',537,'EXECUTED','7:102c49a9dc7c107924ffb5794cefbe3d','createIndex','',NULL,'3.1.0'),('dump13','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',538,'EXECUTED','7:1059694eee38db968447fc0697747d9f','createIndex','',NULL,'3.1.0'),('dump14','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',539,'EXECUTED','7:24f527c8a9d819f8a53ea4d45fbc643a','createIndex','',NULL,'3.1.0'),('dump15','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',540,'EXECUTED','7:92c9ab762efde60ac69733be3b87e5e3','createIndex','',NULL,'3.1.0'),('dump16','yasker (generated)','db/core-035.xml','2015-09-15 23:22:16',541,'EXECUTED','7:d2fffb39d8f2924d1fb65fa2e981d86a','createIndex','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-036.xml','2015-09-15 23:22:16',542,'EXECUTED','7:5453bb5d02ef794d4f98f90419561741','dropForeignKeyConstraint','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-036.xml','2015-09-15 23:22:16',543,'EXECUTED','7:258654f49afb10f1fe77fbcc677be005','dropColumn','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:16',544,'EXECUTED','7:54f71f91e02849dbea28070a4ce35bdc','createTable','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:16',545,'EXECUTED','7:c5a98036b67991d5062cec3f756e18cb','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:16',546,'EXECUTED','7:e38fadfd3e33e6cb3ab5717ee90ce978','addUniqueConstraint','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:17',547,'EXECUTED','7:255c8772375066cb6b155c28a8c68b8f','createIndex','',NULL,'3.1.0'),('dump5','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:17',548,'EXECUTED','7:e240e0bb982c45c834c56b15d47157db','createIndex','',NULL,'3.1.0'),('dump6','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:17',549,'EXECUTED','7:4fdeb2928bc670901e6332d24d106b5b','createIndex','',NULL,'3.1.0'),('dump7','wizardofmath (generated)','db/core-037.xml','2015-09-15 23:22:17',550,'EXECUTED','7:14fec1c3752121d479b27732e45c7075','createIndex','',NULL,'3.1.0'),('dump1','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',551,'EXECUTED','7:d6862420cc1af75550596a2b2160fe20','createTable','',NULL,'3.1.0'),('dump2','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',552,'EXECUTED','7:2682d02a1a1b1ae65a36d60e10e6455d','createTable','',NULL,'3.1.0'),('dump3','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',553,'EXECUTED','7:3b7de7e4cc7173de4ff414516bdfe981','createTable','',NULL,'3.1.0'),('dump4','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',554,'EXECUTED','7:a888c294d1ba764dcadbbfd68dfcbd1e','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',555,'EXECUTED','7:f02c98bb6a8caafe776e0a8104322bc3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',556,'EXECUTED','7:4b88b3111e9ac24c9cedcb12e3e70cbf','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',557,'EXECUTED','7:dac8276e11d5dafc8ab0aa33dfc00c00','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',558,'EXECUTED','7:e08310dbb716dbd2b89f33d3f2a54b51','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',559,'EXECUTED','7:80cf33199ee752d86b9c50e519b48109','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',560,'EXECUTED','7:de3482ab6c968b83a7bec2cd161451d2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump11','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',561,'EXECUTED','7:f9583df602e624d1b7ba0a44de3c157a','addUniqueConstraint','',NULL,'3.1.0'),('dump12','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',562,'EXECUTED','7:1e6ef4a9b65a76dbcde95d4b0e6607dd','addUniqueConstraint','',NULL,'3.1.0'),('dump13','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',563,'EXECUTED','7:1be83735a8255b5e4357a908e7689b81','addUniqueConstraint','',NULL,'3.1.0'),('dump14','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',564,'EXECUTED','7:cacb230ec5ff5249e5fe6bd668986d07','createIndex','',NULL,'3.1.0'),('dump15','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',565,'EXECUTED','7:1147586d77818d317c459ecf42e92d4c','createIndex','',NULL,'3.1.0'),('dump16','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',566,'EXECUTED','7:426d573255e66c08dc7d8ed25df512d6','createIndex','',NULL,'3.1.0'),('dump17','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',567,'EXECUTED','7:ebf853709396e033c9cda7d57081a7eb','createIndex','',NULL,'3.1.0'),('dump18','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',568,'EXECUTED','7:055d407540d7cb5bdeacd34f18b7bdaa','createIndex','',NULL,'3.1.0'),('dump19','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:17',569,'EXECUTED','7:41350d2d89cefb72c5297904618e4fdb','createIndex','',NULL,'3.1.0'),('dump20','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:18',570,'EXECUTED','7:1a9e4000a1d78235c78d09ba3856af91','createIndex','',NULL,'3.1.0'),('dump21','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:18',571,'EXECUTED','7:5971be9bcf1845da35c16551cdcb8411','createIndex','',NULL,'3.1.0'),('dump22','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:18',572,'EXECUTED','7:3d994ea089b17ef413cbc3a39bb979e9','createIndex','',NULL,'3.1.0'),('dump23','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:18',573,'EXECUTED','7:6c5ebda10b8b3db0f7cb674a1a490100','createIndex','',NULL,'3.1.0'),('dump24','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:18',574,'EXECUTED','7:f799bc39a12a1227679e47a961186e8a','createIndex','',NULL,'3.1.0'),('dump25','sonchang (generated)','db/core-038.xml','2015-09-15 23:22:18',575,'EXECUTED','7:15265ffe2bed01a15ba2de1ef903ebe1','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-039.xml','2015-09-15 23:22:18',576,'EXECUTED','7:7ae14f6171a83b7e9cc3b3aee9fc39dc','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-039.xml','2015-09-15 23:22:18',577,'EXECUTED','7:40c597d7a31998a380d2a5bc395ced34','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',578,'EXECUTED','7:9bb16f3aaadf47658d157b8cbe03c90a','createTable','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',579,'EXECUTED','7:2ab6ed5c29c7e8abcc282287a36c0541','addUniqueConstraint','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',580,'EXECUTED','7:5e1156c3003cfc0da909e6556c8c2789','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',581,'EXECUTED','7:b1c8af7a7acc68015483a59527cefe86','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',582,'EXECUTED','7:dc5cc6f4ee1cb3c6ba705c1f1406bd05','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',583,'EXECUTED','7:f516ef1e318e7409a802b63dd122aecb','createIndex','',NULL,'3.1.0'),('dump7','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',584,'EXECUTED','7:8330fff86c72b0be1b5b8d302d1c9d83','createIndex','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',585,'EXECUTED','7:2944087003f1fe5ea57005d707f072b6','createIndex','',NULL,'3.1.0'),('dump9','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',586,'EXECUTED','7:8ee910c5ba8b8725e6e4680f70345893','createIndex','',NULL,'3.1.0'),('dump10','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',587,'EXECUTED','7:81b0d1f24c8d56d2c937d8c9d17d239e','addColumn','',NULL,'3.1.0'),('dump11','alena (generated)','db/core-040.xml','2015-09-15 23:22:18',588,'EXECUTED','7:0b7543484804eae4f1e9e34c95323bfc','addColumn','',NULL,'3.1.0'),('dump12','alena (generated)','db/core-040.xml','2015-09-15 23:22:19',589,'EXECUTED','7:2c7c9e5a7f077b1e6c0febbd764d99c8','addColumn','',NULL,'3.1.0'),('dump13','alena (generated)','db/core-040.xml','2015-09-15 23:22:19',590,'EXECUTED','7:6247ab6654077ab0b6b96dcdaf340827','addForeignKeyConstraint','',NULL,'3.1.0'),('dump14','alena (generated)','db/core-040.xml','2015-09-15 23:22:19',591,'EXECUTED','7:db7a2fa79ba3e582556c429c08b7674f','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-041.xml','2015-09-15 23:22:19',592,'EXECUTED','7:09d0b63cad5a80a95b0ad28885339825','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',593,'EXECUTED','7:f5941c22791ae486ba88879988f66817','createTable','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',594,'EXECUTED','7:e9185f1df418741eca21b5efe21e1612','createTable','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',595,'EXECUTED','7:c58eb43385bcd5b7b58cb0c1a960dd85','addColumn','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',596,'EXECUTED','7:66b25c66edefa38e1f7a032751005eec','addUniqueConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',597,'EXECUTED','7:863aa2a0a563c0b64fda4f11b947b608','addUniqueConstraint','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',598,'EXECUTED','7:026a8bec38bf00e0af73cd87efca9073','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',599,'EXECUTED','7:b43b1d957caa0290b5dacd943052853d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',600,'EXECUTED','7:1b554f61265d7854c7fb9ef758d9fae2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',601,'EXECUTED','7:b2d240d8dfa24a375fb08f93788625c4','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',602,'EXECUTED','7:00e5fadc935ae79ffdb3c67ef5c1c187','createIndex','',NULL,'3.1.0'),('dump11','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',603,'EXECUTED','7:050cdff1eb3313c1016533e17953b094','createIndex','',NULL,'3.1.0'),('dump12','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',604,'EXECUTED','7:64c3211d833e1ebd537bdc07313d29f0','createIndex','',NULL,'3.1.0'),('dump13','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',605,'EXECUTED','7:389c78e5af1afdcc81760cc7e78e544a','createIndex','',NULL,'3.1.0'),('dump14','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',606,'EXECUTED','7:e06008b75745c13bea8974f4df4e4eba','createIndex','',NULL,'3.1.0'),('dump15','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',607,'EXECUTED','7:da581e6efa8efd7a6388c5075c72ef92','createIndex','',NULL,'3.1.0'),('dump16','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',608,'EXECUTED','7:4e18d1d4f79a67290a61080c1070a205','createIndex','',NULL,'3.1.0'),('dump17','alena (generated)','db/core-042.xml','2015-09-15 23:22:19',609,'EXECUTED','7:6e967e7c82294710bc07c309dd665384','createIndex','',NULL,'3.1.0'),('dump18','alena (generated)','db/core-042.xml','2015-09-15 23:22:20',610,'EXECUTED','7:95586390040d080fa8142ca49b1d911a','dropForeignKeyConstraint','',NULL,'3.1.0'),('dump19','alena (generated)','db/core-042.xml','2015-09-15 23:22:20',611,'EXECUTED','7:8b65cd620cf41064944206bdab1f06ca','dropForeignKeyConstraint','',NULL,'3.1.0'),('dump20','alena (generated)','db/core-042.xml','2015-09-15 23:22:20',612,'EXECUTED','7:236e2810d519bbdd9957fc3731107aec','dropForeignKeyConstraint','',NULL,'3.1.0'),('dump21','alena (generated)','db/core-042.xml','2015-09-15 23:22:20',613,'EXECUTED','7:c631bcacb09f509fd29915d2319afce7','dropUniqueConstraint','',NULL,'3.1.0'),('dump22','alena (generated)','db/core-042.xml','2015-09-15 23:22:20',614,'EXECUTED','7:106d341db0d3b32ce8d406553746b3e5','dropTable','',NULL,'3.1.0'),('dump23','alena (generated)','db/core-042.xml','2015-09-15 23:22:20',615,'EXECUTED','7:318e6d30e0a2bd95cc8765372e5fa97d','dropColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-043.xml','2015-09-15 23:22:20',616,'EXECUTED','7:bcb204c49edca2eb2057505c799cbbf1','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-043.xml','2015-09-15 23:22:20',617,'EXECUTED','7:77b0982b62c605f6465aab72fd05281f','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-043.xml','2015-09-15 23:22:20',618,'EXECUTED','7:8bbe7561c1888c12744019211c5e8c67','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-043.xml','2015-09-15 23:22:20',619,'EXECUTED','7:7a8f894ddc56934e56be766ec6034341','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',620,'EXECUTED','7:7b25a9b72be03adaf29f1269d0428310','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',621,'EXECUTED','7:0aff669370bc8580603c79bf51c605ac','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',622,'EXECUTED','7:2a6b2f1ffce78f3dee959c520689851c','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',623,'EXECUTED','7:3b336366396230993e15499d937737fd','addUniqueConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',624,'EXECUTED','7:769eeb40e630a0c94ee14d3481fc73b3','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',625,'EXECUTED','7:08a7768ac1ec21ea5b01ebbcf325be82','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',626,'EXECUTED','7:85818ca4efeee27e9fc46a6c7bf99dca','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',627,'EXECUTED','7:fec454a9a6af0573f2e1ba216824f17f','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',628,'EXECUTED','7:5daea2c6ea73b1f6acb34f81ad755ae2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',629,'EXECUTED','7:240ecba6c25c4f217004ecf53d08ae1f','createIndex','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',630,'EXECUTED','7:1ceece34c6ea3c45e4fb2614c4e23387','createIndex','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',631,'EXECUTED','7:2c5ab785566b7d07a98b114530e4d3b2','createIndex','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-044.xml','2015-09-15 23:22:20',632,'EXECUTED','7:b1c915cc69cc16efc0b4375dea741e52','createIndex','',NULL,'3.1.0'),('dump1','sonchang (generated)','db/core-045.xml','2015-09-15 23:22:20',633,'EXECUTED','7:776292f04c8f9bdc7640778b687c1caf','modifyDataType','',NULL,'3.1.0'),('dump2','sonchang (generated)','db/core-045.xml','2015-09-15 23:22:20',634,'EXECUTED','7:315d468c2e4261fe008f6260af9f3dee','modifyDataType','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-046.xml','2015-09-15 23:22:21',635,'EXECUTED','7:f2d043e450fcbb960d31da3358394a13','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-046.xml','2015-09-15 23:22:21',636,'EXECUTED','7:283de6475768899596c6055ccdd0d299','addColumn','',NULL,'3.1.0'),('sql047','darren','db/core-047.sql','2015-09-15 23:22:21',637,'EXECUTED','7:7be923a9d1ffd7352f39d0f34dde3834','sql','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-048.xml','2015-09-15 23:22:21',638,'EXECUTED','7:2918011c1034e1a84673e5852f4b764e','addUniqueConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-048.xml','2015-09-15 23:22:21',639,'EXECUTED','7:58e639cb2841126328984be6d4be8f58','dropUniqueConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-049.xml','2015-09-15 23:22:21',640,'EXECUTED','7:f1244821d840241efdb1d26c2a57d159','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-050.xml','2015-09-15 23:22:21',641,'EXECUTED','7:43d40c3dea4eab6a36c8739a840493f9','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-051.xml','2015-09-15 23:22:21',642,'EXECUTED','7:eca5b3e5b4a1b1048e12e12e80182f76','addColumn','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-052.xml','2015-09-15 23:22:21',643,'EXECUTED','7:1ed91e4bcf6784801b4ad3adccc80e8e','addColumn','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-052.xml','2015-09-15 23:22:21',644,'EXECUTED','7:b7cf28e5be8eb06d10d6135f38d46611','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',645,'EXECUTED','7:a72458795579e74f2e0e320461cacf5c','modifyDataType','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',646,'EXECUTED','7:f2239a8935ce2a09a701fd0918cc3023','modifyDataType','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',647,'EXECUTED','7:8b86f81e78b19d64d12471394876b214','modifyDataType','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',648,'EXECUTED','7:8a6a3dc427b437f51109a3544911b132','modifyDataType','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',649,'EXECUTED','7:912ca60158a461c9598fd4c15ae1a6ea','modifyDataType','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',650,'EXECUTED','7:24fc632ec6ca3f2b379589c6c90669a9','modifyDataType','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',651,'EXECUTED','7:aeaae7fcc246e3fb9ab850c92e5fac12','modifyDataType','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',652,'EXECUTED','7:d920925210b4133e9a43dd9f0d7d91f5','modifyDataType','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',653,'EXECUTED','7:1e266e17f1502a46813d2c67fbd4314f','modifyDataType','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',654,'EXECUTED','7:64c8fbe6937b8daac53a09696d20b8a6','modifyDataType','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',655,'EXECUTED','7:920349a02fd20258cb4ca5ef4aac13eb','modifyDataType','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',656,'EXECUTED','7:a27f15608cdd5a2f0c76c926fe650b0a','modifyDataType','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',657,'EXECUTED','7:015f8a19b994310dc760f627bb72b7a4','modifyDataType','',NULL,'3.1.0'),('dump19','darren (generated)','db/core-053.xml','2015-09-15 23:22:21',658,'EXECUTED','7:6f6ab07e16dbac351b6128497d8458d6','modifyDataType','',NULL,'3.1.0'),('dump20','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',659,'EXECUTED','7:6749dd062c5ff7952ff933a6712060f2','modifyDataType','',NULL,'3.1.0'),('dump21','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',660,'EXECUTED','7:2a6aff2f14832c1d81da8d2d48371864','modifyDataType','',NULL,'3.1.0'),('dump22','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',661,'EXECUTED','7:8f8fdfd4ee37cad4b8dcca5e3c1b21c8','modifyDataType','',NULL,'3.1.0'),('dump23','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',662,'EXECUTED','7:3d0583c44f546662608ddd4078c8e6c1','modifyDataType','',NULL,'3.1.0'),('dump24','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',663,'EXECUTED','7:3788c32ce21c54ae9c0d49b39903297c','modifyDataType','',NULL,'3.1.0'),('dump25','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',664,'EXECUTED','7:55bf21fb282ec37e57c31d23ce5b51e2','modifyDataType','',NULL,'3.1.0'),('dump26','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',665,'EXECUTED','7:563ea0019c2c1fb6d4d4b1cd45468ac3','modifyDataType','',NULL,'3.1.0'),('dump27','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',666,'EXECUTED','7:38e9adb947528314d79387efb4a4b883','modifyDataType','',NULL,'3.1.0'),('dump28','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',667,'EXECUTED','7:5d60bca5355fdf3512d5395df8d3e619','modifyDataType','',NULL,'3.1.0'),('dump29','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',668,'EXECUTED','7:3a3d7e4aee2d488c7debbab187f09716','modifyDataType','',NULL,'3.1.0'),('dump30','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',669,'EXECUTED','7:25cdf49516419d31fd7579db4dbc3dd5','modifyDataType','',NULL,'3.1.0'),('dump31','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',670,'EXECUTED','7:9f2aa1080055d8275425ab532e671608','modifyDataType','',NULL,'3.1.0'),('dump32','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',671,'EXECUTED','7:d188371b1131a8e79152c4cdc841a7a4','modifyDataType','',NULL,'3.1.0'),('dump33','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',672,'EXECUTED','7:d2405ba0ad1dfcc66968f9069bb9ee7e','modifyDataType','',NULL,'3.1.0'),('dump34','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',673,'EXECUTED','7:c1ad19475947ae0524ffc755c62c94ae','modifyDataType','',NULL,'3.1.0'),('dump35','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',674,'EXECUTED','7:2a711a1465bcec0c1b091d632adb2622','modifyDataType','',NULL,'3.1.0'),('dump36','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',675,'EXECUTED','7:454cf108bdc898b41ca41db4ae350ba1','modifyDataType','',NULL,'3.1.0'),('dump37','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',676,'EXECUTED','7:48c256ecaf171f0ce54bf4966880046c','modifyDataType','',NULL,'3.1.0'),('dump38','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',677,'EXECUTED','7:cb96b9be78e26778fa5b31849f105cb1','modifyDataType','',NULL,'3.1.0'),('dump39','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',678,'EXECUTED','7:07ccddfa1eed36e93996189e3046d260','modifyDataType','',NULL,'3.1.0'),('dump40','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',679,'EXECUTED','7:ca377f92650e5d0e2f193449cd84d278','modifyDataType','',NULL,'3.1.0'),('dump41','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',680,'EXECUTED','7:4a5fd17e75cb13cf874c8ca9dcde24bd','modifyDataType','',NULL,'3.1.0'),('dump42','darren (generated)','db/core-053.xml','2015-09-15 23:22:22',681,'EXECUTED','7:98b002139adc718a4fc2688a3efd83bb','modifyDataType','',NULL,'3.1.0'),('dump43','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',682,'EXECUTED','7:80bee9cf1d3ae87baefae37b15ca6f91','modifyDataType','',NULL,'3.1.0'),('dump44','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',683,'EXECUTED','7:08a203586ce22b1b33dbc78afeb7ccdf','modifyDataType','',NULL,'3.1.0'),('dump45','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',684,'EXECUTED','7:65246789fcdea1697266dd73ef928d76','modifyDataType','',NULL,'3.1.0'),('dump46','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',685,'EXECUTED','7:9b646e3b53f5223d656d8b7cdc04e920','modifyDataType','',NULL,'3.1.0'),('dump47','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',686,'EXECUTED','7:a58e1b24acbc3a5d9974fbf42c6320fa','modifyDataType','',NULL,'3.1.0'),('dump48','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',687,'EXECUTED','7:b2d28342f1559de0a91237cce183fee4','modifyDataType','',NULL,'3.1.0'),('dump49','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',688,'EXECUTED','7:fc25ea847584098746477e92240cdaed','modifyDataType','',NULL,'3.1.0'),('dump50','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',689,'EXECUTED','7:a3588c7c8bde2405f25d8833b9b5da6d','modifyDataType','',NULL,'3.1.0'),('dump51','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',690,'EXECUTED','7:14aa4bc584c42ad21e64f404f6f0eabb','modifyDataType','',NULL,'3.1.0'),('dump52','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',691,'EXECUTED','7:b2c7db9a6c6ef9b5fba7a9d303a3e4ad','modifyDataType','',NULL,'3.1.0'),('dump53','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',692,'EXECUTED','7:afc4fb9379ef880d30f2a2ff97a18f4f','modifyDataType','',NULL,'3.1.0'),('dump54','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',693,'EXECUTED','7:6d8ca4417954f389549ef08cfb20bbee','modifyDataType','',NULL,'3.1.0'),('dump55','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',694,'EXECUTED','7:5e6946c952fa75c2debd905dea1a9fea','modifyDataType','',NULL,'3.1.0'),('dump56','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',695,'EXECUTED','7:27d6161687a2c98261bd5d6844efe40c','modifyDataType','',NULL,'3.1.0'),('dump57','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',696,'EXECUTED','7:4adc308ffb770ce4d72aa076af490e41','modifyDataType','',NULL,'3.1.0'),('dump58','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',697,'EXECUTED','7:d434cbc4978c0c2407c59cbd043dbdd1','modifyDataType','',NULL,'3.1.0'),('dump59','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',698,'EXECUTED','7:17e46807e41c983873e44467c7db6557','modifyDataType','',NULL,'3.1.0'),('dump60','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',699,'EXECUTED','7:9ca5267c3d6e8acbdb582b008e461d40','modifyDataType','',NULL,'3.1.0'),('dump61','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',700,'EXECUTED','7:215e9c87b0ebbe8a6d6f34efe0ada209','modifyDataType','',NULL,'3.1.0'),('dump62','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',701,'EXECUTED','7:ea6249bd79dc1472860ee5fb432d54ce','modifyDataType','',NULL,'3.1.0'),('dump63','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',702,'EXECUTED','7:96c74e03da70b19cf9a42a1a79788c66','modifyDataType','',NULL,'3.1.0'),('dump64','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',703,'EXECUTED','7:ae36767eb28d9dae40ef12b230c93af4','modifyDataType','',NULL,'3.1.0'),('dump65','darren (generated)','db/core-053.xml','2015-09-15 23:22:23',704,'EXECUTED','7:9e7b9fb89b46663b96e5a29149d9aeb2','modifyDataType','',NULL,'3.1.0'),('dump66','darren (generated)','db/core-053.xml','2015-09-15 23:22:24',705,'EXECUTED','7:d5a72d00b41eb12c46b5f65e9a1dee97','modifyDataType','',NULL,'3.1.0'),('dump67','darren (generated)','db/core-053.xml','2015-09-15 23:22:24',706,'EXECUTED','7:cd484367599c9a3c8e9480946a9c3d85','modifyDataType','',NULL,'3.1.0'),('dump68','darren (generated)','db/core-053.xml','2015-09-15 23:22:24',707,'EXECUTED','7:5cfa9b38668134e004ecad7902c1be1f','modifyDataType','',NULL,'3.1.0'),('dump69','darren (generated)','db/core-053.xml','2015-09-15 23:22:24',708,'EXECUTED','7:43aa000ef7f3c2e7428ca0393993d033','modifyDataType','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-054.xml','2015-09-15 23:22:24',709,'EXECUTED','7:6ae07e94f10df681d4de4f881da59844','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-055.xml','2015-09-15 23:22:24',710,'EXECUTED','7:51121ce33d84adaa4cb1dfaf1dcb4b54','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-056.xml','2015-09-15 23:22:24',711,'EXECUTED','7:0abdbfd68768c0b800f6f72732e1cff9','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',712,'EXECUTED','7:9101d4470c722aa876436d0da104e719','createTable','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',713,'EXECUTED','7:c527cb610eb1ce819a6b827e41079f12','addUniqueConstraint','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',714,'EXECUTED','7:804fe1524a5d773db865d7d621dbb764','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',715,'EXECUTED','7:ca58a7df9ee124470b90c963bd127d75','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',716,'EXECUTED','7:4313d5f4e405794d680294b6673f1cb9','addForeignKeyConstraint','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',717,'EXECUTED','7:5c75c66712b95f88c56754be5c80d155','createIndex','',NULL,'3.1.0'),('dump7','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',718,'EXECUTED','7:ed171f106a95811e67c527ee79d743f4','createIndex','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',719,'EXECUTED','7:5c1a967560d5cf7e1adf6e05f772e321','createIndex','',NULL,'3.1.0'),('dump9','alena (generated)','db/core-057.xml','2015-09-15 23:22:24',720,'EXECUTED','7:f1f138b16f3027fc0a6e06065c8970a3','createIndex','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-058.xml','2015-09-15 23:22:24',721,'EXECUTED','7:1c6342383d257fbad953668b0a264d3c','createTable','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-058.xml','2015-09-15 23:22:24',722,'EXECUTED','7:e46d51381d1c8e046126d2faa1b64a9b','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-058.xml','2015-09-15 23:22:25',723,'EXECUTED','7:092351896667747a799a6c0e91cc7758','createIndex','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-058.xml','2015-09-15 23:22:25',724,'EXECUTED','7:5e5662331fe48725e305573a9e420261','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-059.xml','2015-09-15 23:22:25',725,'EXECUTED','7:3077b37bec0caef0a2a1c038f1a75d7d','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-059.xml','2015-09-15 23:22:25',726,'EXECUTED','7:5fbbde205ddf659f45f532aa3dc3a09c','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-060.xml','2015-09-15 23:22:25',727,'EXECUTED','7:46c16c76490ce496b22a9faa6bf6db0c','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-061.xml','2016-11-29 21:08:12',728,'EXECUTED','7:8f85b3e13f2bd1311e9b4b1d087844fc','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-062.xml','2016-11-29 21:08:12',729,'EXECUTED','7:fc43ea0245b75b43b80ef2e3f3803bdb','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-062.xml','2016-11-29 21:08:13',730,'EXECUTED','7:03c108510e55e7819ecbe81140878f6f','addColumn','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-062.xml','2016-11-29 21:08:13',731,'EXECUTED','7:a48cda6c3aa1af942e21a24cec69c7d4','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-063.xml','2016-11-29 21:08:13',732,'EXECUTED','7:7cda1004d89354c701da52a6226ef9b4','addColumn','',NULL,'3.1.0'),('1444429809024-1','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:13',733,'EXECUTED','7:914b4e795a683843736ff92d812e0747','createTable','',NULL,'3.1.0'),('1444429809024-2','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:14',734,'EXECUTED','7:a371e364b45fe3e118fe14b4b0dd6b37','addColumn','',NULL,'3.1.0'),('1444429809024-3','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:14',735,'EXECUTED','7:325b75af628291717f6f426d3171a690','addColumn','',NULL,'3.1.0'),('1444429809024-4','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:14',736,'EXECUTED','7:54d6efcb6135e1b9e4ce13a9e9c643c5','addUniqueConstraint','',NULL,'3.1.0'),('1444429809024-5','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:14',737,'EXECUTED','7:51cac4d46c52a347a3236838c6228e04','addForeignKeyConstraint','',NULL,'3.1.0'),('1444429809024-6','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:14',738,'EXECUTED','7:b2b6b63bc2f66c86843c34647edd95d2','addForeignKeyConstraint','',NULL,'3.1.0'),('1444429809024-7','cjellick (generated)','db/core-064.xml','2016-11-29 21:08:15',739,'EXECUTED','7:981f2a7413b771ac5f766f062e80971f','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-065.xml','2016-11-29 21:08:15',740,'EXECUTED','7:3d30822225e22ac4cf9e371e2a50f554','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-065.xml','2016-11-29 21:08:15',741,'EXECUTED','7:203b302174f612d1fff8de47aac83e72','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-066.xml','2016-11-29 21:08:15',742,'EXECUTED','7:8dc27d8bbbd5915fe5bdae6152d16695','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-067.xml','2016-11-29 21:08:15',743,'EXECUTED','7:2d5e2c30b868ec3d61fce571f5692e81','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-067.xml','2016-11-29 21:08:15',744,'EXECUTED','7:b3bd011eb551aefb8b6aaf2f8388eedc','addUniqueConstraint','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-067.xml','2016-11-29 21:08:16',745,'EXECUTED','7:b20ff2d4d939affba2dfb52a8dce92b7','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-067.xml','2016-11-29 21:08:16',746,'EXECUTED','7:bd4684797dbddc00e2d0068d6e376e49','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-067.xml','2016-11-29 21:08:16',747,'EXECUTED','7:48266ed8c41e982f2bb901f6d00ee487','createIndex','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-067.xml','2016-11-29 21:08:16',748,'EXECUTED','7:3fb253afdfcc4e334cced44eb509c9ee','createIndex','',NULL,'3.1.0'),('1442449895671-4','cjellick (generated)','db/core-068.xml','2016-11-29 21:08:16',749,'EXECUTED','7:610874cb788603f79e23af74559c983c','addColumn','',NULL,'3.1.0'),('1442449895671-16','cjellick (generated)','db/core-068.xml','2016-11-29 21:08:16',750,'EXECUTED','7:7dde09be75cd4ef26cb2b7f917ddbbaf','createIndex','',NULL,'3.1.0'),('1446752802832-1','sidharthamani (generated)','db/core-069.xml','2016-11-29 21:08:17',751,'EXECUTED','7:2e806cb522403948be510e27a1bcba0a','addColumn','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-070.xml','2016-11-29 21:08:17',752,'EXECUTED','7:6ed88482b03744cd868d0e52d2e8f2ac','createTable','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-070.xml','2016-11-29 21:08:17',753,'EXECUTED','7:6a1e18bae6ba591f06288a3dfda04d42','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-070.xml','2016-11-29 21:08:17',754,'EXECUTED','7:cdd98487dfece8bba71f7592c3964ac0','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-070.xml','2016-11-29 21:08:17',755,'EXECUTED','7:009e322269a1c9710063cb500291164a','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-071.xml','2016-11-29 21:08:17',756,'EXECUTED','7:53efad1551661cd89dc7f902bf052590','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-072.xml','2016-11-29 21:08:18',757,'EXECUTED','7:2100eb9c7a82e255324ac058ea253101','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-072.xml','2016-11-29 21:08:18',758,'EXECUTED','7:2ef4b723f72fb8d8b5c7a1330fa58d93','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-073.xml','2016-11-29 21:08:18',759,'EXECUTED','7:bee07963a47877a752a05ed61178d0d9','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-074.xml','2016-11-29 21:08:19',760,'EXECUTED','7:eddb88af30b84d30e34701173c810028','createTable','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-074.xml','2016-11-29 21:08:19',761,'EXECUTED','7:38895b070fed829013e2dd03b231b5dd','addUniqueConstraint','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-074.xml','2016-11-29 21:08:19',762,'EXECUTED','7:f8748bbdbff6f5ad9a9e10d37014a45d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-074.xml','2016-11-29 21:08:19',763,'EXECUTED','7:e114228f1407f0c2866b4ac2d03adeb4','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-074.xml','2016-11-29 21:08:19',764,'EXECUTED','7:70ae550f334e36e5f988d93d4a645152','createIndex','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-074.xml','2016-11-29 21:08:19',765,'EXECUTED','7:d8778bee2e1ef6a9bb4fc51088b7c9a7','createIndex','',NULL,'3.1.0'),('dump7','alena (generated)','db/core-074.xml','2016-11-29 21:08:20',766,'EXECUTED','7:3263c71c17faca6250369266fce9f462','createIndex','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-074.xml','2016-11-29 21:08:20',767,'EXECUTED','7:caa4e2fc43ab36dc4e2ed1acf88af24d','createIndex','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:20',768,'EXECUTED','7:11b6652023d6976ad657bc60e7f47837','createTable','',NULL,'3.1.0'),('dump2','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:20',769,'EXECUTED','7:52b39202d1808eeda053be9a4834bed9','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:20',770,'EXECUTED','7:7715a8739e82e6196897e49a83a734ba','addUniqueConstraint','',NULL,'3.1.0'),('dump4','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:20',771,'EXECUTED','7:61a10117e57e03e9b4c170824c4c150e','createIndex','',NULL,'3.1.0'),('dump5','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:20',772,'EXECUTED','7:ce4929612c47e17a0bfe90b12edb867f','createIndex','',NULL,'3.1.0'),('dump6','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:20',773,'EXECUTED','7:d53a45b17ebfde394158dfc62f1dec1a','createIndex','',NULL,'3.1.0'),('dump7','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:21',774,'EXECUTED','7:97a12893100045fa930e72c3414ad392','createIndex','',NULL,'3.1.0'),('dump8','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:21',775,'EXECUTED','7:f596429c5907e1e5caf27aa908ec8a8f','createTable','',NULL,'3.1.0'),('dump9','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:21',776,'EXECUTED','7:ba5774913cfcd281e505f9e38da8ab12','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','wizardofmath (generated)','db/core-075.xml','2016-11-29 21:08:21',777,'EXECUTED','7:4c9c20168f48c94e0626432e80b553da','addColumn','',NULL,'3.1.0'),('dump1','wizardofmath (generated)','db/core-076.xml','2016-11-29 21:08:22',778,'EXECUTED','7:937333ad5099d755b6f49a2a082c4de3','addColumn','',NULL,'3.1.0'),('dump9','wizardofmath (generated)','db/core-076.xml','2016-11-29 21:08:22',779,'EXECUTED','7:f84d314457f5fb4bdd21ee845b843a16','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-077.xml','2016-11-29 21:08:22',780,'EXECUTED','7:0733e7166cf961e8f0d9f861c87609aa','createIndex','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-077.xml','2016-11-29 21:08:22',781,'EXECUTED','7:f318dc5a1bff455232ace27c15d8da96','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-078.xml','2016-11-29 21:08:22',782,'EXECUTED','7:cc8d54d8a8766baaf2023737b1f2449f','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-079.xml','2016-11-29 21:08:22',783,'EXECUTED','7:d82ec495666679b8653507d6d898e36d','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-079.xml','2016-11-29 21:08:23',784,'EXECUTED','7:6b372475b94ee0eb39d1d3ac8e5176c2','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-080.xml','2016-11-29 21:08:23',785,'EXECUTED','7:b25791233e27e84a094bdd7917fe6fb7','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-080.xml','2016-11-29 21:08:23',786,'EXECUTED','7:e9a59ee9cb3e3d79e80546644bc9cbf5','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-081.xml','2016-11-29 21:08:23',787,'EXECUTED','7:38be2eb91867eb8301ed833eaa04fa91','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-081.xml','2016-11-29 21:08:24',788,'EXECUTED','7:16b5762d8ad923f10f3b4e8b1c5e0ef2','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-082.xml','2016-11-29 21:08:24',789,'EXECUTED','7:17162c49cdf3f900c6bafb8d88054a2d','modifyDataType','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-083.xml','2016-11-29 21:08:24',790,'EXECUTED','7:2fe54afcea5626c16689d69ddfbbc58c','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-083.xml','2016-11-29 21:08:25',791,'EXECUTED','7:c7b1e05df45a73030fed03c23970e22e','addColumn','',NULL,'3.1.0'),('dump1','prachi (generated)','db/core-084.xml','2016-11-29 21:08:25',792,'EXECUTED','7:6553e4aca329a5d3ab1472811e533e8a','createIndex','',NULL,'3.1.0'),('1460757751518-1','rancher','db/core-085.xml','2016-11-29 21:08:25',793,'EXECUTED','7:7dee1ca9d030454c0ccc23c284d73b70','addColumn','',NULL,'3.1.0'),('1460757751518-2','rancher','db/core-085.xml','2016-11-29 21:08:26',794,'EXECUTED','7:7b4e213ca60b68c7924875c547442437','addColumn','',NULL,'3.1.0'),('1460757751518-3','rancher','db/core-085.xml','2016-11-29 21:08:26',795,'EXECUTED','7:0d62c0fb614e829ff4aae2fdbaa1b084','addColumn','',NULL,'3.1.0'),('1460757751518-4','rancher','db/core-085.xml','2016-11-29 21:08:26',796,'EXECUTED','7:63ca4069f36b059eadd9673684d445b6','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-086.xml','2016-11-29 21:08:26',797,'EXECUTED','7:6f25d47b5e75487aae7e4d69ef9a6928','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-086.xml','2016-11-29 21:08:27',798,'EXECUTED','7:16004b3ae80c580b3d294929c153a8fa','createIndex','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-087.xml','2016-11-29 21:08:27',799,'EXECUTED','7:bf04ab653fced4b747c17fd16c5962cd','dropNotNullConstraint','',NULL,'3.1.0'),('1464210595168-1','Rancher Labs','db/core-088.xml','2016-11-29 21:08:27',800,'EXECUTED','7:f6361c09788bb0808b58f00a89e34aab','createTable','',NULL,'3.1.0'),('1464210595168-2','Rancher Labs','db/core-088.xml','2016-11-29 21:08:27',801,'EXECUTED','7:af25358b9d0c2640fc9c0ee8faa1d407','createTable','',NULL,'3.1.0'),('1464210595168-3','Rancher Labs','db/core-088.xml','2016-11-29 21:08:27',802,'EXECUTED','7:ae347460f0396b1dbff7893891e0fa11','addUniqueConstraint','',NULL,'3.1.0'),('1464210595168-4','Rancher Labs','db/core-088.xml','2016-11-29 21:08:27',803,'EXECUTED','7:74af09b02a1f6ce4793a027079af3a84','addUniqueConstraint','',NULL,'3.1.0'),('1464210595168-5','Rancher Labs','db/core-088.xml','2016-11-29 21:08:27',804,'EXECUTED','7:e68526728ea1798b70662c38a053957e','addForeignKeyConstraint','',NULL,'3.1.0'),('1464210595168-6','Rancher Labs','db/core-088.xml','2016-11-29 21:08:28',805,'EXECUTED','7:8141b553c420dd57314814b9fd09b05a','addForeignKeyConstraint','',NULL,'3.1.0'),('1464210595168-7','Rancher Labs','db/core-088.xml','2016-11-29 21:08:28',806,'EXECUTED','7:4567846731e266d047b5653ee588355c','addForeignKeyConstraint','',NULL,'3.1.0'),('1464210595168-8','Rancher Labs','db/core-088.xml','2016-11-29 21:08:28',807,'EXECUTED','7:3092513adcd163f545851424f93ff397','addForeignKeyConstraint','',NULL,'3.1.0'),('1464210595168-9','Rancher Labs','db/core-088.xml','2016-11-29 21:08:28',808,'EXECUTED','7:1614fad4a151c24c44f88e13b864aab4','addForeignKeyConstraint','',NULL,'3.1.0'),('1464210595168-10','Rancher Labs','db/core-088.xml','2016-11-29 21:08:28',809,'EXECUTED','7:6f7879784fc7a8933229999aad3a2e7d','addForeignKeyConstraint','',NULL,'3.1.0'),('1464210595168-11','Rancher Labs','db/core-088.xml','2016-11-29 21:08:28',810,'EXECUTED','7:451bef29abc4e3dd8a125d1f7cb91e8e','createIndex','',NULL,'3.1.0'),('1464210595168-12','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',811,'EXECUTED','7:cd011dacd23b0b340a25eb990d036b3c','createIndex','',NULL,'3.1.0'),('1464210595168-13','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',812,'EXECUTED','7:2f618187f1437395867f9de2f495ad94','createIndex','',NULL,'3.1.0'),('1464210595168-14','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',813,'EXECUTED','7:bbdc58ab90552165ea16d1ddde74aa6e','createIndex','',NULL,'3.1.0'),('1464210595168-15','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',814,'EXECUTED','7:5084d508170419ea59e4158152f03809','createIndex','',NULL,'3.1.0'),('1464210595168-16','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',815,'EXECUTED','7:e511837c5ec36e1503161845818c90ef','createIndex','',NULL,'3.1.0'),('1464210595168-17','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',816,'EXECUTED','7:7e9508ed9e1c7b05787710ef500e3043','createIndex','',NULL,'3.1.0'),('1464210595168-18','Rancher Labs','db/core-088.xml','2016-11-29 21:08:29',817,'EXECUTED','7:243d27509ec93f903bd696dff577a954','createIndex','',NULL,'3.1.0'),('1465274608991-1','Rancher Labs','db/core-089.xml','2016-11-29 21:08:29',818,'EXECUTED','7:4766dc3e01c7bb22ec3ef383525d8dac','dropForeignKeyConstraint','',NULL,'3.1.0'),('1465274608991-2','Rancher Labs','db/core-089.xml','2016-11-29 21:08:30',819,'EXECUTED','7:72dd9dde940cc5aeafb32090419259a7','dropColumn','',NULL,'3.1.0'),('1465274608991-3','Rancher Labs','db/core-089.xml','2016-11-29 21:08:30',820,'EXECUTED','7:b8a7b9e0f3b1f786867901580a97296f','dropColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-090.xml','2016-11-29 21:08:30',821,'EXECUTED','7:4a862000356b2a8b8e8699749fdf7cf5','addColumn','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-090.xml','2016-11-29 21:08:30',822,'EXECUTED','7:8a9c5f423f4abe0731d4c815af003cd1','addForeignKeyConstraint','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-091.xml','2016-11-29 21:08:31',823,'EXECUTED','7:ca6bee506cadbbb6d85cf7fbd1633b96','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-092.xml','2016-11-29 21:08:31',824,'EXECUTED','7:e80ac36edfab547994d3c6813d6a7bd4','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-092.xml','2016-11-29 21:08:31',825,'EXECUTED','7:1b17ef8ac8512ee8dcd0672023e788fc','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-092.xml','2016-11-29 21:08:31',826,'EXECUTED','7:dd14e2874ff3fc76bbf509197cdf3f69','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-092.xml','2016-11-29 21:08:31',827,'EXECUTED','7:2ba88ce67a8349c0a47e1575773f9777','addForeignKeyConstraint','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-093.xml','2016-11-29 21:08:32',828,'EXECUTED','7:e9c55e33022ce626bed31519f95559a3','addColumn','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-094.xml','2016-11-29 21:08:32',829,'EXECUTED','7:877299cbe54fb29b1bcc3a350d88b8af','createTable','',NULL,'3.1.0'),('dump2','alena (generated)','db/core-094.xml','2016-11-29 21:08:32',830,'EXECUTED','7:3963582c8c167c0d83cd3116d7e20912','addUniqueConstraint','',NULL,'3.1.0'),('dump3','alena (generated)','db/core-094.xml','2016-11-29 21:08:32',831,'EXECUTED','7:f4aa92bce225a57e241decf250365886','addForeignKeyConstraint','',NULL,'3.1.0'),('dump4','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',832,'EXECUTED','7:3fa296dbd79e7630dcf5e338c77aa3e4','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',833,'EXECUTED','7:e0c9a5919d13e12eb6ef8c6e50162c3c','createIndex','',NULL,'3.1.0'),('dump6','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',834,'EXECUTED','7:28a6175f76955660026b676c300525d5','createIndex','',NULL,'3.1.0'),('dump7','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',835,'EXECUTED','7:f62c64f9d74dcab5aee3e004056d3181','createIndex','',NULL,'3.1.0'),('dump8','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',836,'EXECUTED','7:1feb52a3ce754d95972398700b6db26d','createIndex','',NULL,'3.1.0'),('dump9','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',837,'EXECUTED','7:7257bb5f634bf889da5095b4507c542d','createTable','',NULL,'3.1.0'),('dump10','alena (generated)','db/core-094.xml','2016-11-29 21:08:33',838,'EXECUTED','7:5065154173324c6facd094e3fd91a72c','addColumn','',NULL,'3.1.0'),('dump11','alena (generated)','db/core-094.xml','2016-11-29 21:08:34',839,'EXECUTED','7:ea3fcbba8a1acbc77a76b2a6e1071a5d','addColumn','',NULL,'3.1.0'),('dump12','alena (generated)','db/core-094.xml','2016-11-29 21:08:34',840,'EXECUTED','7:bab5838fe10d35d2e76649d822cd2949','addColumn','',NULL,'3.1.0'),('dump13','alena (generated)','db/core-094.xml','2016-11-29 21:08:34',841,'EXECUTED','7:1b329571549e7c0ba32f3995d4afef74','addUniqueConstraint','',NULL,'3.1.0'),('dump14','alena (generated)','db/core-094.xml','2016-11-29 21:08:35',842,'EXECUTED','7:94867187dfa80c1caffd715614d609b4','addForeignKeyConstraint','',NULL,'3.1.0'),('dump15','alena (generated)','db/core-094.xml','2016-11-29 21:08:35',843,'EXECUTED','7:6ca2693458afda0596f3ba8f65220376','addForeignKeyConstraint','',NULL,'3.1.0'),('dump16','alena (generated)','db/core-094.xml','2016-11-29 21:08:35',844,'EXECUTED','7:8af9eec5f5cbf444dd2c13a1a11b2700','addForeignKeyConstraint','',NULL,'3.1.0'),('dump17','alena (generated)','db/core-094.xml','2016-11-29 21:08:36',845,'EXECUTED','7:b41fd38523d6919e4a6523e274312107','addForeignKeyConstraint','',NULL,'3.1.0'),('dump18','alena (generated)','db/core-094.xml','2016-11-29 21:08:36',846,'EXECUTED','7:e6706ca32b97b8f45a31d300e0de1c2d','addForeignKeyConstraint','',NULL,'3.1.0'),('dump19','alena (generated)','db/core-094.xml','2016-11-29 21:08:36',847,'EXECUTED','7:aa891ec5cdbdf87f8b9297ef3a405221','createIndex','',NULL,'3.1.0'),('dump20','alena (generated)','db/core-094.xml','2016-11-29 21:08:36',848,'EXECUTED','7:080cf191cb83a67c12ad4ec1bd9cc6c4','createIndex','',NULL,'3.1.0'),('dump21','alena (generated)','db/core-094.xml','2016-11-29 21:08:36',849,'EXECUTED','7:f83c0a750b43ff3f21c641275257885f','createIndex','',NULL,'3.1.0'),('dump22','alena (generated)','db/core-094.xml','2016-11-29 21:08:36',850,'EXECUTED','7:648c5be239e801f2b6b501a4403c068c','createIndex','',NULL,'3.1.0'),('dump1','alena (generated)','db/core-095.xml','2016-11-29 21:08:37',851,'EXECUTED','7:317ff6bdd9232be4ac55c67f5a013861','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-096.xml','2016-11-29 21:08:37',852,'EXECUTED','7:effbedb6dddddbd9b2d64b5731d3ccc4','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-096.xml','2016-11-29 21:08:37',853,'EXECUTED','7:ebd4af534fe7917f33cc53946d1225b4','createTable','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-096.xml','2016-11-29 21:08:37',854,'EXECUTED','7:c0a8e12c1038b02be704968e0a488446','addColumn','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-096.xml','2016-11-29 21:08:37',855,'EXECUTED','7:64a9bc201fedc31919916fade0234964','addUniqueConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-096.xml','2016-11-29 21:08:37',856,'EXECUTED','7:a33780f5b977436b73abc108372da671','addUniqueConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-096.xml','2016-11-29 21:08:37',857,'EXECUTED','7:746f811c0bf1b9eeab4e54838cd09e10','addForeignKeyConstraint','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-096.xml','2016-11-29 21:08:38',858,'EXECUTED','7:3aa50eff76c926968cfa2c73349c8345','addForeignKeyConstraint','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-096.xml','2016-11-29 21:08:38',859,'EXECUTED','7:878f42b440020f4b6508cf1f0e4d8531','addForeignKeyConstraint','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-096.xml','2016-11-29 21:08:38',860,'EXECUTED','7:8586fae3451837480351428ec2b29074','addForeignKeyConstraint','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-096.xml','2016-11-29 21:08:38',861,'EXECUTED','7:4c57c7edbd73d9071d3790df0a551cb6','addForeignKeyConstraint','',NULL,'3.1.0'),('dump13','darren (generated)','db/core-096.xml','2016-11-29 21:08:38',862,'EXECUTED','7:6a45d42b6779832acb8b8f74d09f9e42','createIndex','',NULL,'3.1.0'),('dump14','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',863,'EXECUTED','7:e6c4ba06a30d26179c81ee96129c3a22','createIndex','',NULL,'3.1.0'),('dump15','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',864,'EXECUTED','7:84d08274705c57131378f297ff932f97','createIndex','',NULL,'3.1.0'),('dump16','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',865,'EXECUTED','7:d3c1ac035632184fcc4d04d629be0ba7','createIndex','',NULL,'3.1.0'),('dump17','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',866,'EXECUTED','7:3a82091106fde93b13aaccdcc28121f5','createIndex','',NULL,'3.1.0'),('dump18','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',867,'EXECUTED','7:1560ed4f8f29e149e393a1638315ea47','createIndex','',NULL,'3.1.0'),('dump19','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',868,'EXECUTED','7:a1700fd436c0f5c820e6550022ef2d11','createIndex','',NULL,'3.1.0'),('dump20','darren (generated)','db/core-096.xml','2016-11-29 21:08:39',869,'EXECUTED','7:79eef14272f04349daf5c515c4864c85','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-097.xml','2016-11-29 21:08:40',870,'EXECUTED','7:7f4efeec47b178108b2bc917567b3dea','addColumn','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-097.xml','2016-11-29 21:08:40',871,'EXECUTED','7:e28b717871a5e4e2b35d8045902dbcdc','addForeignKeyConstraint','',NULL,'3.1.0'),('1475017154185-1','Rancher Labs','db/core-098.xml','2016-11-29 21:08:40',872,'EXECUTED','7:79907000c504a02aabe244b8e03a0a40','addColumn','',NULL,'3.1.0'),('1475017154185-2','Rancher Labs','db/core-098.xml','2016-11-29 21:08:41',873,'EXECUTED','7:d992b40cd880232893a00c4493454d34','addColumn','',NULL,'3.1.0'),('1475017154185-3','Rancher Labs','db/core-098.xml','2016-11-29 21:08:41',874,'EXECUTED','7:d067b9b85a7dc319504fd64b1dcaef14','addColumn','',NULL,'3.1.0'),('1475017154185-4','Rancher Labs','db/core-098.xml','2016-11-29 21:08:41',875,'EXECUTED','7:f77020fab85547a5c18754113b1e6125','addColumn','',NULL,'3.1.0'),('1475017154185-5','Rancher Labs','db/core-098.xml','2016-11-29 21:08:42',876,'EXECUTED','7:ae741747fb4a33aef654668079d73eee','addColumn','',NULL,'3.1.0'),('1475017154185-6','Rancher Labs','db/core-098.xml','2016-11-29 21:08:42',877,'EXECUTED','7:0cb974a558c579e3d415bb0506282216','addColumn','',NULL,'3.1.0'),('dump1','rajashree (generated)','db/core-099.xml','2016-11-29 21:08:42',878,'EXECUTED','7:6bef30932337db218ec51d5937364212','modifyDataType','',NULL,'3.1.0'),('dump2','rajashree (generated)','db/core-099.xml','2016-11-29 21:08:43',879,'EXECUTED','7:d3505656e80394cef9d3985c89018bd3','addNotNullConstraint','',NULL,'3.1.0'),('dump1','Rancher (generated)','db/core-100.xml','2016-11-29 21:08:43',880,'EXECUTED','7:f881474402daacb39243bf3c56ad3e17','addColumn','',NULL,'3.1.0'),('dump2','Rancher (generated)','db/core-100.xml','2016-11-29 21:08:43',881,'EXECUTED','7:7c9f70e31e43d1408127113e2781d895','addColumn','',NULL,'3.1.0'),('dump3','Rancher (generated)','db/core-100.xml','2016-11-29 21:08:43',882,'EXECUTED','7:aa68393d1c066cb38db9dcb8ab67f8d7','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-102.xml','2016-11-29 21:08:44',883,'EXECUTED','7:90721cdbb2933903222d9d37fe16f711','createTable','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-102.xml','2016-11-29 21:08:44',884,'EXECUTED','7:fd5c2af503d24abe8ff59274ba4a719d','addColumn','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-102.xml','2016-11-29 21:08:44',885,'EXECUTED','7:44cf0942c405ae10dbabeddd9763ad89','addUniqueConstraint','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-102.xml','2016-11-29 21:08:44',886,'EXECUTED','7:742644e33b80aa4eb802c9541c31f583','addForeignKeyConstraint','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-102.xml','2016-11-29 21:08:44',887,'EXECUTED','7:bad3160c4ba8946c0c0df1ae9bea6f7c','addForeignKeyConstraint','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-102.xml','2016-11-29 21:08:45',888,'EXECUTED','7:81a0076e76742915a15b1f45307aca95','createIndex','',NULL,'3.1.0'),('dump9','darren (generated)','db/core-102.xml','2016-11-29 21:08:45',889,'EXECUTED','7:c59c2247ec1a981243237c047fa3f8d3','createIndex','',NULL,'3.1.0'),('dump10','darren (generated)','db/core-102.xml','2016-11-29 21:08:45',890,'EXECUTED','7:ca2abe52e2722a2c65ac66123514a18e','createIndex','',NULL,'3.1.0'),('dump11','darren (generated)','db/core-102.xml','2016-11-29 21:08:45',891,'EXECUTED','7:7dc536ee1ccf83a715be5c4644754074','createIndex','',NULL,'3.1.0'),('dump12','darren (generated)','db/core-102.xml','2016-11-29 21:08:45',892,'EXECUTED','7:53e43d2ba6df48f91aac4e5e7f617088','createIndex','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-103.xml','2016-11-29 21:08:45',893,'EXECUTED','7:25adb8bc374b29acfd2deb9a203e666e','addColumn','',NULL,'3.1.0'),('dump4','darren (generated)','db/core-103.xml','2016-11-29 21:08:46',894,'EXECUTED','7:89c2413e2cc5762b675fde216efc9d2f','addForeignKeyConstraint','',NULL,'3.1.0'),('dump5','darren (generated)','db/core-103.xml','2016-11-29 21:08:46',895,'EXECUTED','7:df2d1f9e5c4d727cc95b30b6827b974c','addColumn','',NULL,'3.1.0'),('dump6','darren (generated)','db/core-103.xml','2016-11-29 21:08:46',896,'EXECUTED','7:755947e259809c64fba7621593af9645','addColumn','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-103.xml','2016-11-29 21:08:46',897,'EXECUTED','7:e51055e3663984823dd324abe9a19006','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-104.xml','2016-11-29 21:08:46',898,'EXECUTED','7:12f591f71568fc9bb533ccfaf2bf592e','addColumn','',NULL,'3.1.0'),('dump7','darren (generated)','db/core-104.xml','2016-11-29 21:08:47',899,'EXECUTED','7:9f5e152ac282f340fb2a4722f5e05b45','addColumn','',NULL,'3.1.0'),('dump8','darren (generated)','db/core-104.xml','2016-11-29 21:08:47',900,'EXECUTED','7:05b45d522827f751db85eb5fdb66f0e7','createIndex','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-105.xml','2016-11-29 21:08:47',901,'EXECUTED','7:d353cb073469919b901305f975328be5','addColumn','',NULL,'3.1.0'),('dump1','darren (generated)','db/core-106.xml','2016-11-29 21:08:47',902,'EXECUTED','7:074b4767754359f4646fc29d69efd60a','createTable','',NULL,'3.1.0'),('dump2','darren (generated)','db/core-106.xml','2016-11-29 21:08:47',903,'EXECUTED','7:0039e3a35f21c552c549229d32c13811','addUniqueConstraint','',NULL,'3.1.0'),('dump3','darren (generated)','db/core-106.xml','2016-11-29 21:08:47',904,'EXECUTED','7:7a256aaad6635e6312e04a0b7af5f2c7','createIndex','',NULL,'3.1.0');
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
  `health_state` varchar(128) DEFAULT NULL,
  `project_template_id` bigint(20) DEFAULT NULL,
  `default_network_id` bigint(20) DEFAULT NULL,
  `version` varchar(128) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_account_uuid` (`uuid`),
  KEY `idx_account_name` (`name`),
  KEY `idx_account_remove_time` (`remove_time`),
  KEY `idx_account_removed` (`removed`),
  KEY `idx_account_state` (`state`),
  KEY `idx_external_ids` (`external_id`,`external_id_type`),
  KEY `fk_account__project_template_id` (`project_template_id`),
  CONSTRAINT `fk_account__project_template_id` FOREIGN KEY (`project_template_id`) REFERENCES `project_template` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `account`
--

LOCK TABLES `account` WRITE;
/*!40000 ALTER TABLE `account` DISABLE KEYS */;
INSERT INTO `account` VALUES (1,'admin','admin','admin',NULL,'active',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL),(2,'system','system','system',NULL,'inactive',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL),(3,'superadmin','superadmin','superadmin',NULL,'inactive',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL);
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
  `managed_config` bit(1) NOT NULL DEFAULT b'1',
  `agent_group_id` bigint(20) DEFAULT NULL,
  `zone_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_agent_uuid` (`uuid`),
  KEY `fk_agent__account_id` (`account_id`),
  KEY `fk_agent__agent_group_id` (`agent_group_id`),
  KEY `fk_agent__zone_id` (`zone_id`),
  KEY `idx_agent_name` (`name`),
  KEY `idx_agent_remove_time` (`remove_time`),
  KEY `idx_agent_removed` (`removed`),
  KEY `idx_agent_state` (`state`),
  KEY `fk_agent__uri` (`uri`),
  CONSTRAINT `fk_agent__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_agent__agent_group_id` FOREIGN KEY (`agent_group_id`) REFERENCES `agent_group` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_agent__zone_id` FOREIGN KEY (`zone_id`) REFERENCES `zone` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `agent_group`
--

DROP TABLE IF EXISTS `agent_group`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `agent_group` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_agent_group_uuid` (`uuid`),
  KEY `fk_agent_group__account_id` (`account_id`),
  KEY `idx_agent_group_name` (`name`),
  KEY `idx_agent_group_remove_time` (`remove_time`),
  KEY `idx_agent_group_removed` (`removed`),
  KEY `idx_agent_group_state` (`state`),
  CONSTRAINT `fk_agent_group__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `agent_group`
--

LOCK TABLES `agent_group` WRITE;
/*!40000 ALTER TABLE `agent_group` DISABLE KEYS */;
/*!40000 ALTER TABLE `agent_group` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `key` (`key`),
  UNIQUE KEY `idx_auth_token_key` (`key`),
  KEY `fk_auth_token__account_id` (`account_id`),
  KEY `idx_auth_token_expires` (`expires`),
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
-- Table structure for table `backup`
--

DROP TABLE IF EXISTS `backup`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `backup` (
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
  `snapshot_id` bigint(20) DEFAULT NULL,
  `volume_id` bigint(20) DEFAULT NULL,
  `backup_target_id` bigint(20) DEFAULT NULL,
  `uri` varchar(4096) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_backup_uuid` (`uuid`),
  KEY `fk_backup__account_id` (`account_id`),
  KEY `fk_backup__backup_target_id` (`backup_target_id`),
  KEY `fk_backup__snapshot_id` (`snapshot_id`),
  KEY `fk_backup__volume_id` (`volume_id`),
  KEY `idx_backup_name` (`name`),
  KEY `idx_backup_remove_time` (`remove_time`),
  KEY `idx_backup_removed` (`removed`),
  KEY `idx_backup_state` (`state`),
  CONSTRAINT `fk_backup__volume_id` FOREIGN KEY (`volume_id`) REFERENCES `volume` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_backup__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_backup__backup_target_id` FOREIGN KEY (`backup_target_id`) REFERENCES `backup_target` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_backup__snapshot_id` FOREIGN KEY (`snapshot_id`) REFERENCES `snapshot` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `backup`
--

LOCK TABLES `backup` WRITE;
/*!40000 ALTER TABLE `backup` DISABLE KEYS */;
/*!40000 ALTER TABLE `backup` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `backup_target`
--

DROP TABLE IF EXISTS `backup_target`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `backup_target` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_backup_target_uuid` (`uuid`),
  KEY `fk_backup_target__account_id` (`account_id`),
  KEY `idx_backup_target_name` (`name`),
  KEY `idx_backup_target_remove_time` (`remove_time`),
  KEY `idx_backup_target_removed` (`removed`),
  KEY `idx_backup_target_state` (`state`),
  CONSTRAINT `fk_backup_target__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `backup_target`
--

LOCK TABLES `backup_target` WRITE;
/*!40000 ALTER TABLE `backup_target` DISABLE KEYS */;
/*!40000 ALTER TABLE `backup_target` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cert_data_uuid` (`uuid`),
  KEY `fk_cert_data__account_id` (`account_id`),
  KEY `idx_cert_data_name` (`name`),
  KEY `idx_cert_data_remove_time` (`remove_time`),
  KEY `idx_cert_data_removed` (`removed`),
  KEY `idx_cert_data_state` (`state`),
  CONSTRAINT `fk_cert_data__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `cluster_host_map`
--

DROP TABLE IF EXISTS `cluster_host_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `cluster_host_map` (
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
  `cluster_id` bigint(20) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_host_map_uuid` (`uuid`),
  KEY `fk_cluster_host_map__host_id` (`cluster_id`),
  KEY `fk_host_host_map__host_id` (`host_id`),
  KEY `idx_cluster_host_map_name` (`name`),
  KEY `idx_cluster_host_map_remove_time` (`remove_time`),
  KEY `idx_cluster_host_map_removed` (`removed`),
  KEY `idx_cluster_host_map_state` (`state`),
  CONSTRAINT `fk_cluster_host_map__host_id` FOREIGN KEY (`cluster_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host_host_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `cluster_host_map`
--

LOCK TABLES `cluster_host_map` WRITE;
/*!40000 ALTER TABLE `cluster_host_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `cluster_host_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `cluster_membership`
--

DROP TABLE IF EXISTS `cluster_membership`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `cluster_membership` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `uuid` varchar(128) NOT NULL,
  `heartbeat` bigint(20) DEFAULT NULL,
  `config` mediumtext,
  `clustered` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cluster_membership_uuid` (`uuid`),
  KEY `idx_cluster_membership_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `cluster_membership`
--

LOCK TABLES `cluster_membership` WRITE;
/*!40000 ALTER TABLE `cluster_membership` DISABLE KEYS */;
/*!40000 ALTER TABLE `cluster_membership` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `config_item`
--

DROP TABLE IF EXISTS `config_item`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `config_item` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `source_version` varchar(1024) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_config_item__name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `config_item`
--

LOCK TABLES `config_item` WRITE;
/*!40000 ALTER TABLE `config_item` DISABLE KEYS */;
/*!40000 ALTER TABLE `config_item` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `config_item_status`
--

DROP TABLE IF EXISTS `config_item_status`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `config_item_status` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `requested_version` bigint(20) NOT NULL DEFAULT '0',
  `applied_version` bigint(20) NOT NULL DEFAULT '-1',
  `source_version` varchar(255) DEFAULT NULL,
  `requested_updated` datetime NOT NULL,
  `applied_updated` datetime DEFAULT NULL,
  `agent_id` bigint(20) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  `resource_id` bigint(20) NOT NULL,
  `resource_type` varchar(128) NOT NULL,
  `environment_id` bigint(20) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_config_item_status_resource` (`name`,`resource_type`,`resource_id`),
  KEY `fk_config_item__agent_id` (`agent_id`),
  KEY `idx_config_item_source_version` (`source_version`),
  KEY `fk_config_item__account_id` (`account_id`),
  KEY `fk_config_item__service_id` (`service_id`),
  KEY `idx_config_item_status__resource_id` (`resource_id`),
  KEY `fk_config_item__environment_id` (`environment_id`),
  KEY `fk_config_item__host_id` (`host_id`),
  CONSTRAINT `fk_config_item__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_config_item__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_config_item__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_config_item__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `environment` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_config_item__name` FOREIGN KEY (`name`) REFERENCES `config_item` (`name`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_config_item__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `config_item_status`
--

LOCK TABLES `config_item_status` WRITE;
/*!40000 ALTER TABLE `config_item_status` DISABLE KEYS */;
/*!40000 ALTER TABLE `config_item_status` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `container_event`
--

DROP TABLE IF EXISTS `container_event`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `container_event` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `data` mediumtext,
  `external_id` varchar(255) DEFAULT NULL,
  `external_status` varchar(255) DEFAULT NULL,
  `external_from` varchar(255) DEFAULT NULL,
  `external_timestamp` bigint(20) DEFAULT NULL,
  `reported_host_uuid` varchar(255) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_container_event__account_id` (`account_id`),
  KEY `fk_container_event__host_id` (`host_id`),
  KEY `idx_container_event_created` (`created`),
  KEY `idx_container_event_external_timestamp` (`external_timestamp`),
  KEY `idx_container_event_state` (`state`),
  CONSTRAINT `fk_container_event__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_container_event__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `container_event`
--

LOCK TABLES `container_event` WRITE;
/*!40000 ALTER TABLE `container_event` DISABLE KEYS */;
/*!40000 ALTER TABLE `container_event` ENABLE KEYS */;
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
  KEY `idx_credential_name` (`name`),
  KEY `idx_credential_remove_time` (`remove_time`),
  KEY `idx_credential_removed` (`removed`),
  KEY `idx_credential_state` (`state`),
  KEY `fk_credential__registry_id` (`registry_id`),
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
-- Table structure for table `credential_instance_map`
--

DROP TABLE IF EXISTS `credential_instance_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `credential_instance_map` (
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
  `credential_id` bigint(20) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_credential_instance_map_uuid` (`uuid`),
  KEY `fk_credential_instance_map__credential_id` (`credential_id`),
  KEY `fk_credential_instance_map__instance_id` (`instance_id`),
  KEY `idx_credential_instance_map_name` (`name`),
  KEY `idx_credential_instance_map_remove_time` (`remove_time`),
  KEY `idx_credential_instance_map_removed` (`removed`),
  KEY `idx_credential_instance_map_state` (`state`),
  CONSTRAINT `fk_credential_instance_map__credential_id` FOREIGN KEY (`credential_id`) REFERENCES `credential` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_credential_instance_map__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `credential_instance_map`
--

LOCK TABLES `credential_instance_map` WRITE;
/*!40000 ALTER TABLE `credential_instance_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `credential_instance_map` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_deployment_unit_uuid` (`uuid`),
  KEY `fk_deployment_unit__account_id` (`account_id`),
  KEY `fk_deployment_unit__service_id` (`service_id`),
  KEY `idx_deployment_unit_name` (`name`),
  KEY `idx_deployment_unit_remove_time` (`remove_time`),
  KEY `idx_deployment_unit_removed` (`removed`),
  KEY `idx_deployment_unit_state` (`state`),
  CONSTRAINT `fk_deployment_unit__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_deployment_unit__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_dynamic_schema_uuid` (`uuid`),
  KEY `fk_dynamic_schema__account_id` (`account_id`),
  KEY `fk_dynamic_schema__service_id` (`service_id`),
  KEY `idx_dynamic_schema_name` (`name`),
  KEY `idx_dynamic_schema_state` (`state`),
  KEY `idx_dynamic_schema_removed` (`removed`),
  CONSTRAINT `fk_dynamic_schema__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
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
-- Table structure for table `environment`
--

DROP TABLE IF EXISTS `environment`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `environment` (
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
  `system` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_environment_uuid` (`uuid`),
  KEY `fk_environment__account_id` (`account_id`),
  KEY `idx_environment_name` (`name`),
  KEY `idx_environment_remove_time` (`remove_time`),
  KEY `idx_environment_removed` (`removed`),
  KEY `idx_environment_state` (`state`),
  KEY `idx_environment_external_id` (`external_id`),
  CONSTRAINT `fk_environment__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `environment`
--

LOCK TABLES `environment` WRITE;
/*!40000 ALTER TABLE `environment` DISABLE KEYS */;
/*!40000 ALTER TABLE `environment` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_external_event_uuid` (`uuid`),
  KEY `fk_external_event__account_id` (`account_id`),
  KEY `fk_external_event__reported_account_id` (`reported_account_id`),
  KEY `idx_external_event_state` (`state`),
  CONSTRAINT `fk_external_event__reported_account_id` FOREIGN KEY (`reported_account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_external_event__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `external_handler`
--

DROP TABLE IF EXISTS `external_handler`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `external_handler` (
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
  `priority` int(11) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_external_handler_uuid` (`uuid`),
  KEY `idx_external_handler_name` (`name`),
  KEY `idx_external_handler_remove_time` (`remove_time`),
  KEY `idx_external_handler_removed` (`removed`),
  KEY `idx_external_handler_state` (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `external_handler`
--

LOCK TABLES `external_handler` WRITE;
/*!40000 ALTER TABLE `external_handler` DISABLE KEYS */;
/*!40000 ALTER TABLE `external_handler` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `external_handler_external_handler_process_map`
--

DROP TABLE IF EXISTS `external_handler_external_handler_process_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `external_handler_external_handler_process_map` (
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
  `external_handler_id` bigint(20) DEFAULT NULL,
  `external_handler_process_id` bigint(20) DEFAULT NULL,
  `on_error` varchar(255) DEFAULT NULL,
  `event_name` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_eh_eh_process_map_uuid` (`uuid`),
  KEY `fk_eh_eh_process_map__external_handler_id` (`external_handler_id`),
  KEY `fk_eh_eh_process_map__external_handler_process_id` (`external_handler_process_id`),
  KEY `idx_eh_eh_process_map_name` (`name`),
  KEY `idx_eh_eh_process_map_remove_time` (`remove_time`),
  KEY `idx_eh_eh_process_map_removed` (`removed`),
  KEY `idx_eh_eh_process_map_state` (`state`),
  CONSTRAINT `fk_eh_eh_process_map__external_handler_id` FOREIGN KEY (`external_handler_id`) REFERENCES `external_handler` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_eh_eh_process_map__external_handler_process_id` FOREIGN KEY (`external_handler_process_id`) REFERENCES `external_handler_process` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `external_handler_external_handler_process_map`
--

LOCK TABLES `external_handler_external_handler_process_map` WRITE;
/*!40000 ALTER TABLE `external_handler_external_handler_process_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `external_handler_external_handler_process_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `external_handler_process`
--

DROP TABLE IF EXISTS `external_handler_process`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `external_handler_process` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_external_handler_process_uuid` (`uuid`),
  KEY `idx_external_handler_process_name` (`name`),
  KEY `idx_external_handler_process_remove_time` (`remove_time`),
  KEY `idx_external_handler_process_removed` (`removed`),
  KEY `idx_external_handler_process_state` (`state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `external_handler_process`
--

LOCK TABLES `external_handler_process` WRITE;
/*!40000 ALTER TABLE `external_handler_process` DISABLE KEYS */;
/*!40000 ALTER TABLE `external_handler_process` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_generic_object_uuid` (`uuid`),
  KEY `fk_generic_object__account_id` (`account_id`),
  KEY `idx_generic_object_key` (`key`),
  KEY `idx_generic_object_name` (`name`),
  KEY `idx_generic_object_remove_time` (`remove_time`),
  KEY `idx_generic_object_removed` (`removed`),
  KEY `idx_generic_object_state` (`state`),
  CONSTRAINT `fk_generic_object__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `global_load_balancer`
--

DROP TABLE IF EXISTS `global_load_balancer`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `global_load_balancer` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_global_load_balancer_uuid` (`uuid`),
  KEY `fk_global_load_balancer__account_id` (`account_id`),
  KEY `idx_global_load_balancer_name` (`name`),
  KEY `idx_global_load_balancer_remove_time` (`remove_time`),
  KEY `idx_global_load_balancer_removed` (`removed`),
  KEY `idx_global_load_balancer_state` (`state`),
  CONSTRAINT `fk_global_load_balancer__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `global_load_balancer`
--

LOCK TABLES `global_load_balancer` WRITE;
/*!40000 ALTER TABLE `global_load_balancer` DISABLE KEYS */;
/*!40000 ALTER TABLE `global_load_balancer` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `healthcheck_instance`
--

DROP TABLE IF EXISTS `healthcheck_instance`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `healthcheck_instance` (
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
  `instance_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_healthcheck_instance_uuid` (`uuid`),
  KEY `fk_healthcheck_instance__account_id` (`account_id`),
  KEY `idx_healthcheck_instance_name` (`name`),
  KEY `idx_healthcheck_instance_remove_time` (`remove_time`),
  KEY `idx_healthcheck_instance_removed` (`removed`),
  KEY `idx_healthcheck_instance_state` (`state`),
  KEY `fk_healthcheck_instance__instance_id` (`instance_id`),
  CONSTRAINT `fk_healthcheck_instance__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_healthcheck_instance__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `healthcheck_instance`
--

LOCK TABLES `healthcheck_instance` WRITE;
/*!40000 ALTER TABLE `healthcheck_instance` DISABLE KEYS */;
/*!40000 ALTER TABLE `healthcheck_instance` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `healthcheck_instance_host_map`
--

DROP TABLE IF EXISTS `healthcheck_instance_host_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `healthcheck_instance_host_map` (
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
  `healthcheck_instance_id` bigint(20) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  `external_timestamp` bigint(20) DEFAULT NULL,
  `health_state` varchar(128) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_healthcheck_instance_host_map_uuid` (`uuid`),
  KEY `fk_healthcheck_instance_host_map__account_id` (`account_id`),
  KEY `fk_healthcheck_instance_host_map__healthcheck_instance_id` (`healthcheck_instance_id`),
  KEY `fk_healthcheck_instance_host_map__host_id` (`host_id`),
  KEY `idx_healthcheck_instance_host_map_name` (`name`),
  KEY `idx_healthcheck_instance_host_map_remove_time` (`remove_time`),
  KEY `idx_healthcheck_instance_host_map_removed` (`removed`),
  KEY `idx_healthcheck_instance_host_map_state` (`state`),
  KEY `fk_healthcheck_instance_host_map_instance_id` (`instance_id`),
  CONSTRAINT `fk_healthcheck_instance_host_map_instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_healthcheck_instance_host_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_healthcheck_instance_host_map__healthcheck_instance_id` FOREIGN KEY (`healthcheck_instance_id`) REFERENCES `healthcheck_instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_healthcheck_instance_host_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `healthcheck_instance_host_map`
--

LOCK TABLES `healthcheck_instance_host_map` WRITE;
/*!40000 ALTER TABLE `healthcheck_instance_host_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `healthcheck_instance_host_map` ENABLE KEYS */;
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
  `compute_free` bigint(20) DEFAULT NULL,
  `compute_total` bigint(20) DEFAULT NULL,
  `agent_id` bigint(20) DEFAULT NULL,
  `zone_id` bigint(20) DEFAULT NULL,
  `physical_host_id` bigint(20) DEFAULT NULL,
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  `agent_state` varchar(128) DEFAULT NULL,
  `local_storage_mb` bigint(20) DEFAULT NULL,
  `memory` bigint(20) DEFAULT NULL,
  `milli_cpu` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_host_uuid` (`uuid`),
  KEY `fk_host__account_id` (`account_id`),
  KEY `fk_host__agent_id` (`agent_id`),
  KEY `fk_host__zone_id` (`zone_id`),
  KEY `idx_host_compute_free` (`compute_free`),
  KEY `idx_host_name` (`name`),
  KEY `idx_host_remove_time` (`remove_time`),
  KEY `idx_host_removed` (`removed`),
  KEY `idx_host_state` (`state`),
  KEY `fk_host__physical_host_id` (`physical_host_id`),
  KEY `idx_host_is_public` (`is_public`),
  CONSTRAINT `fk_host__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__physical_host_id` FOREIGN KEY (`physical_host_id`) REFERENCES `physical_host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host__zone_id` FOREIGN KEY (`zone_id`) REFERENCES `zone` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `host_ip_address_map`
--

DROP TABLE IF EXISTS `host_ip_address_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `host_ip_address_map` (
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
  `host_id` bigint(20) DEFAULT NULL,
  `ip_address_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_host_ip_address_map_uuid` (`uuid`),
  KEY `fk_host_ip_address_map__host_id` (`host_id`),
  KEY `fk_host_ip_address_map__ip_address_id` (`ip_address_id`),
  KEY `idx_host_ip_address_map_name` (`name`),
  KEY `idx_host_ip_address_map_remove_time` (`remove_time`),
  KEY `idx_host_ip_address_map_removed` (`removed`),
  KEY `idx_host_ip_address_map_state` (`state`),
  CONSTRAINT `fk_host_ip_address_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host_ip_address_map__ip_address_id` FOREIGN KEY (`ip_address_id`) REFERENCES `ip_address` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `host_ip_address_map`
--

LOCK TABLES `host_ip_address_map` WRITE;
/*!40000 ALTER TABLE `host_ip_address_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `host_ip_address_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `host_label_map`
--

DROP TABLE IF EXISTS `host_label_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `host_label_map` (
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
  `label_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_host_label_map_uuid` (`uuid`),
  KEY `fk_host_label_map__account_id` (`account_id`),
  KEY `fk_host_label_map__host_id` (`host_id`),
  KEY `fk_host_label_map__label_id` (`label_id`),
  KEY `idx_host_label_map_name` (`name`),
  KEY `idx_host_label_map_remove_time` (`remove_time`),
  KEY `idx_host_label_map_removed` (`removed`),
  KEY `idx_host_label_map_state` (`state`),
  CONSTRAINT `fk_host_label_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host_label_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host_label_map__label_id` FOREIGN KEY (`label_id`) REFERENCES `label` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `host_label_map`
--

LOCK TABLES `host_label_map` WRITE;
/*!40000 ALTER TABLE `host_label_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `host_label_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `host_vnet_map`
--

DROP TABLE IF EXISTS `host_vnet_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `host_vnet_map` (
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
  `host_id` bigint(20) DEFAULT NULL,
  `vnet_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_host_vnet_map_uuid` (`uuid`),
  KEY `fk_host_vnet_map__host_id` (`host_id`),
  KEY `fk_host_vnet_map__vnet_id` (`vnet_id`),
  KEY `idx_host_vnet_map_name` (`name`),
  KEY `idx_host_vnet_map_remove_time` (`remove_time`),
  KEY `idx_host_vnet_map_removed` (`removed`),
  KEY `idx_host_vnet_map_state` (`state`),
  CONSTRAINT `fk_host_vnet_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_host_vnet_map__vnet_id` FOREIGN KEY (`vnet_id`) REFERENCES `vnet` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `host_vnet_map`
--

LOCK TABLES `host_vnet_map` WRITE;
/*!40000 ALTER TABLE `host_vnet_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `host_vnet_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `image`
--

DROP TABLE IF EXISTS `image`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `image` (
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
  `url` varchar(255) DEFAULT NULL,
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  `physical_size_mb` bigint(20) DEFAULT NULL,
  `virtual_size_mb` bigint(20) DEFAULT NULL,
  `checksum` varchar(255) DEFAULT NULL,
  `format` varchar(255) DEFAULT NULL,
  `instance_kind` varchar(255) DEFAULT NULL,
  `registry_credential_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_image_uuid` (`uuid`),
  KEY `fk_image__account_id` (`account_id`),
  KEY `idx_image_name` (`name`),
  KEY `idx_image_remove_time` (`remove_time`),
  KEY `idx_image_removed` (`removed`),
  KEY `idx_image_state` (`state`),
  KEY `idx_image_instance_kind` (`instance_kind`),
  KEY `fk_image_registry_credential_id` (`registry_credential_id`),
  CONSTRAINT `fk_image_registry_credential_id` FOREIGN KEY (`registry_credential_id`) REFERENCES `credential` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_image__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `image`
--

LOCK TABLES `image` WRITE;
/*!40000 ALTER TABLE `image` DISABLE KEYS */;
/*!40000 ALTER TABLE `image` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `image_storage_pool_map`
--

DROP TABLE IF EXISTS `image_storage_pool_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `image_storage_pool_map` (
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
  `image_id` bigint(20) DEFAULT NULL,
  `storage_pool_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_image_storage_pool_map_uuid` (`uuid`),
  KEY `fk_image_storage_pool_map__image_id` (`image_id`),
  KEY `fk_image_storage_pool_map__storage_pool_id` (`storage_pool_id`),
  KEY `idx_image_storage_pool_map_name` (`name`),
  KEY `idx_image_storage_pool_map_remove_time` (`remove_time`),
  KEY `idx_image_storage_pool_map_removed` (`removed`),
  KEY `idx_image_storage_pool_map_state` (`state`),
  CONSTRAINT `fk_image_storage_pool_map__image_id` FOREIGN KEY (`image_id`) REFERENCES `image` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_image_storage_pool_map__storage_pool_id` FOREIGN KEY (`storage_pool_id`) REFERENCES `storage_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `image_storage_pool_map`
--

LOCK TABLES `image_storage_pool_map` WRITE;
/*!40000 ALTER TABLE `image_storage_pool_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `image_storage_pool_map` ENABLE KEYS */;
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
  `allocation_state` varchar(255) DEFAULT NULL,
  `compute` bigint(20) DEFAULT NULL,
  `memory_mb` bigint(20) DEFAULT NULL,
  `image_id` bigint(20) DEFAULT NULL,
  `offering_id` bigint(20) DEFAULT NULL,
  `hostname` varchar(255) DEFAULT NULL,
  `zone_id` bigint(20) DEFAULT NULL,
  `instance_triggered_stop` varchar(128) DEFAULT NULL,
  `agent_id` bigint(20) DEFAULT NULL,
  `domain` varchar(128) DEFAULT NULL,
  `first_running` datetime DEFAULT NULL,
  `token` varchar(255) DEFAULT NULL,
  `userdata` text,
  `system_container` varchar(128) DEFAULT NULL,
  `registry_credential_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT NULL,
  `native_container` bit(1) NOT NULL DEFAULT b'0',
  `network_container_id` bigint(20) DEFAULT NULL,
  `health_state` varchar(128) DEFAULT NULL,
  `start_count` bigint(20) DEFAULT NULL,
  `create_index` bigint(20) DEFAULT NULL,
  `deployment_unit_uuid` varchar(128) DEFAULT NULL,
  `version` varchar(255) DEFAULT '0',
  `health_updated` datetime DEFAULT NULL,
  `service_index_id` bigint(20) DEFAULT NULL,
  `dns_internal` varchar(255) DEFAULT NULL,
  `dns_search_internal` varchar(255) DEFAULT NULL,
  `memory_reservation` bigint(20) DEFAULT NULL,
  `milli_cpu_reservation` bigint(20) DEFAULT NULL,
  `system` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_instance_uuid` (`uuid`),
  KEY `fk_instance__account_id` (`account_id`),
  KEY `fk_instance__image_id` (`image_id`),
  KEY `fk_instance__offering_id` (`offering_id`),
  KEY `fk_instance__zone_id` (`zone_id`),
  KEY `idx_instance_name` (`name`),
  KEY `idx_instance_remove_time` (`remove_time`),
  KEY `idx_instance_removed` (`removed`),
  KEY `idx_instance_state` (`state`),
  KEY `fk_instance__agent_id` (`agent_id`),
  KEY `fk_instance__registry_credential_id` (`registry_credential_id`),
  KEY `idx_instance_external_id` (`external_id`),
  KEY `fk_instance__instance_id` (`network_container_id`),
  KEY `fk_instance__service_index_id` (`service_index_id`),
  CONSTRAINT `fk_instance__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__image_id` FOREIGN KEY (`image_id`) REFERENCES `image` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__instance_id` FOREIGN KEY (`network_container_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__offering_id` FOREIGN KEY (`offering_id`) REFERENCES `offering` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__registry_credential_id` FOREIGN KEY (`registry_credential_id`) REFERENCES `credential` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__service_index_id` FOREIGN KEY (`service_index_id`) REFERENCES `service_index` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance__zone_id` FOREIGN KEY (`zone_id`) REFERENCES `zone` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `instance_host_map`
--

DROP TABLE IF EXISTS `instance_host_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `instance_host_map` (
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
  `instance_id` bigint(20) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_instance_host_map_uuid` (`uuid`),
  KEY `fk_instance_host_map__host_id` (`host_id`),
  KEY `fk_instance_host_map__instance_id` (`instance_id`),
  KEY `idx_instance_host_map_name` (`name`),
  KEY `idx_instance_host_map_remove_time` (`remove_time`),
  KEY `idx_instance_host_map_removed` (`removed`),
  KEY `idx_instance_host_map_state` (`state`),
  CONSTRAINT `fk_instance_host_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance_host_map__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `instance_host_map`
--

LOCK TABLES `instance_host_map` WRITE;
/*!40000 ALTER TABLE `instance_host_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `instance_host_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `instance_label_map`
--

DROP TABLE IF EXISTS `instance_label_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `instance_label_map` (
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
  `instance_id` bigint(20) DEFAULT NULL,
  `label_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_instance_label_map_uuid` (`uuid`),
  KEY `fk_instance_label_map__account_id` (`account_id`),
  KEY `fk_instance_label_map__instance_id` (`instance_id`),
  KEY `fk_instance_label_map__label_id` (`label_id`),
  KEY `idx_instance_label_map_name` (`name`),
  KEY `idx_instance_label_map_remove_time` (`remove_time`),
  KEY `idx_instance_label_map_removed` (`removed`),
  KEY `idx_instance_label_map_state` (`state`),
  CONSTRAINT `fk_instance_label_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance_label_map__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_instance_label_map__label_id` FOREIGN KEY (`label_id`) REFERENCES `label` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `instance_label_map`
--

LOCK TABLES `instance_label_map` WRITE;
/*!40000 ALTER TABLE `instance_label_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `instance_label_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `instance_link`
--

DROP TABLE IF EXISTS `instance_link`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `instance_link` (
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
  `link_name` varchar(255) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `target_instance_id` bigint(20) DEFAULT NULL,
  `service_consume_map_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_link_uuid` (`uuid`),
  KEY `fk_link__account_id` (`account_id`),
  KEY `fk_link__instance_id` (`instance_id`),
  KEY `fk_link__target_instance_id` (`target_instance_id`),
  KEY `idx_link_name` (`name`),
  KEY `idx_link_remove_time` (`remove_time`),
  KEY `idx_link_removed` (`removed`),
  KEY `idx_link_state` (`state`),
  KEY `fk_link__service_consume_map_id` (`service_consume_map_id`),
  CONSTRAINT `fk_link__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_link__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_link__service_consume_map_id` FOREIGN KEY (`service_consume_map_id`) REFERENCES `service_consume_map` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_link__target_instance_id` FOREIGN KEY (`target_instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `instance_link`
--

LOCK TABLES `instance_link` WRITE;
/*!40000 ALTER TABLE `instance_link` DISABLE KEYS */;
/*!40000 ALTER TABLE `instance_link` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ip_address`
--

DROP TABLE IF EXISTS `ip_address`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ip_address` (
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
  `address` varchar(255) DEFAULT NULL,
  `subnet_id` bigint(20) DEFAULT NULL,
  `network_id` bigint(20) DEFAULT NULL,
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  `role` varchar(128) DEFAULT NULL,
  `hostname` varchar(255) DEFAULT NULL,
  `ip_pool_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ip_address_uuid` (`uuid`),
  KEY `fk_ip_address__account_id` (`account_id`),
  KEY `fk_ip_address__network_id` (`network_id`),
  KEY `fk_ip_address__subnet_id` (`subnet_id`),
  KEY `idx_ip_address_name` (`name`),
  KEY `idx_ip_address_remove_time` (`remove_time`),
  KEY `idx_ip_address_removed` (`removed`),
  KEY `idx_ip_address_state` (`state`),
  KEY `idx_ip_address_is_public` (`is_public`),
  KEY `fk_ip_address__ip_pool_id` (`ip_pool_id`),
  CONSTRAINT `fk_ip_address__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_address__ip_pool_id` FOREIGN KEY (`ip_pool_id`) REFERENCES `ip_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_address__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_address__subnet_id` FOREIGN KEY (`subnet_id`) REFERENCES `subnet` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ip_address`
--

LOCK TABLES `ip_address` WRITE;
/*!40000 ALTER TABLE `ip_address` DISABLE KEYS */;
/*!40000 ALTER TABLE `ip_address` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ip_address_nic_map`
--

DROP TABLE IF EXISTS `ip_address_nic_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ip_address_nic_map` (
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
  `ip_address_id` bigint(20) DEFAULT NULL,
  `nic_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ip_address_nic_map_uuid` (`uuid`),
  KEY `fk_ip_address_nic_map__ip_address_id` (`ip_address_id`),
  KEY `fk_ip_address_nic_map__nic_id` (`nic_id`),
  KEY `idx_ip_address_nic_map_name` (`name`),
  KEY `idx_ip_address_nic_map_remove_time` (`remove_time`),
  KEY `idx_ip_address_nic_map_removed` (`removed`),
  KEY `idx_ip_address_nic_map_state` (`state`),
  CONSTRAINT `fk_ip_address_nic_map__ip_address_id` FOREIGN KEY (`ip_address_id`) REFERENCES `ip_address` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_address_nic_map__nic_id` FOREIGN KEY (`nic_id`) REFERENCES `nic` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ip_address_nic_map`
--

LOCK TABLES `ip_address_nic_map` WRITE;
/*!40000 ALTER TABLE `ip_address_nic_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `ip_address_nic_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ip_association`
--

DROP TABLE IF EXISTS `ip_association`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ip_association` (
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
  `ip_address_id` bigint(20) DEFAULT NULL,
  `child_ip_address_id` bigint(20) DEFAULT NULL,
  `role` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ip_association_uuid` (`uuid`),
  KEY `fk_ip_association__account_id` (`account_id`),
  KEY `fk_ip_association__child_ip_address_id` (`child_ip_address_id`),
  KEY `fk_ip_association__ip_address_id` (`ip_address_id`),
  KEY `idx_ip_association_name` (`name`),
  KEY `idx_ip_association_remove_time` (`remove_time`),
  KEY `idx_ip_association_removed` (`removed`),
  KEY `idx_ip_association_state` (`state`),
  CONSTRAINT `fk_ip_association__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_association__child_ip_address_id` FOREIGN KEY (`child_ip_address_id`) REFERENCES `ip_address` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_association__ip_address_id` FOREIGN KEY (`ip_address_id`) REFERENCES `ip_address` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ip_association`
--

LOCK TABLES `ip_association` WRITE;
/*!40000 ALTER TABLE `ip_association` DISABLE KEYS */;
/*!40000 ALTER TABLE `ip_association` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `ip_pool`
--

DROP TABLE IF EXISTS `ip_pool`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `ip_pool` (
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
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_ip_pool_uuid` (`uuid`),
  KEY `fk_ip_pool__account_id` (`account_id`),
  KEY `idx_ip_pool_name` (`name`),
  KEY `idx_ip_pool_remove_time` (`remove_time`),
  KEY `idx_ip_pool_removed` (`removed`),
  KEY `idx_ip_pool_state` (`state`),
  CONSTRAINT `fk_ip_pool__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `ip_pool`
--

LOCK TABLES `ip_pool` WRITE;
/*!40000 ALTER TABLE `ip_pool` DISABLE KEYS */;
/*!40000 ALTER TABLE `ip_pool` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `label`
--

DROP TABLE IF EXISTS `label`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `label` (
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
  `type` varchar(255) DEFAULT NULL,
  `key` varchar(1024) DEFAULT NULL,
  `value` varchar(4096) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_label_uuid` (`uuid`),
  KEY `fk_label__account_id` (`account_id`),
  KEY `idx_label_name` (`name`),
  KEY `idx_label_remove_time` (`remove_time`),
  KEY `idx_label_removed` (`removed`),
  KEY `idx_label_state` (`state`),
  KEY `idx_label_key_value` (`key`(255),`value`(255)),
  CONSTRAINT `fk_label__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `label`
--

LOCK TABLES `label` WRITE;
/*!40000 ALTER TABLE `label` DISABLE KEYS */;
/*!40000 ALTER TABLE `label` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer`
--

DROP TABLE IF EXISTS `load_balancer`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer` (
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
  `global_load_balancer_id` bigint(20) DEFAULT NULL,
  `weight` bigint(20) DEFAULT NULL,
  `load_balancer_config_id` bigint(20) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_uuid` (`uuid`),
  KEY `fk_load_balancer__account_id` (`account_id`),
  KEY `fk_load_balancer__global_load_balancer_id` (`global_load_balancer_id`),
  KEY `idx_load_balancer_name` (`name`),
  KEY `idx_load_balancer_remove_time` (`remove_time`),
  KEY `idx_load_balancer_removed` (`removed`),
  KEY `idx_load_balancer_state` (`state`),
  KEY `fk_load_balancer__load_balancer_config_id` (`load_balancer_config_id`),
  KEY `fk_load_balancer__service_id` (`service_id`),
  CONSTRAINT `fk_load_balancer__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer__global_load_balancer_id` FOREIGN KEY (`global_load_balancer_id`) REFERENCES `global_load_balancer` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer__load_balancer_config_id` FOREIGN KEY (`load_balancer_config_id`) REFERENCES `load_balancer_config` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer`
--

LOCK TABLES `load_balancer` WRITE;
/*!40000 ALTER TABLE `load_balancer` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer_certificate_map`
--

DROP TABLE IF EXISTS `load_balancer_certificate_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer_certificate_map` (
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
  `load_balancer_id` bigint(20) DEFAULT NULL,
  `certificate_id` bigint(20) DEFAULT NULL,
  `is_default` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_certificate_map_uuid` (`uuid`),
  KEY `fk_load_balancer_certificate_map__account_id` (`account_id`),
  KEY `fk_load_balancer_certificate_map__certificate_id` (`certificate_id`),
  KEY `fk_load_balancer_certificate_map__load_balancer_id` (`load_balancer_id`),
  KEY `idx_load_balancer_certificate_map_name` (`name`),
  KEY `idx_load_balancer_certificate_map_remove_time` (`remove_time`),
  KEY `idx_load_balancer_certificate_map_removed` (`removed`),
  KEY `idx_load_balancer_certificate_map_state` (`state`),
  CONSTRAINT `fk_load_balancer_certificate_map__load_balancer_id` FOREIGN KEY (`load_balancer_id`) REFERENCES `load_balancer` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_certificate_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_certificate_map__certificate_id` FOREIGN KEY (`certificate_id`) REFERENCES `certificate` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer_certificate_map`
--

LOCK TABLES `load_balancer_certificate_map` WRITE;
/*!40000 ALTER TABLE `load_balancer_certificate_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer_certificate_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer_config`
--

DROP TABLE IF EXISTS `load_balancer_config`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer_config` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_config_uuid` (`uuid`),
  KEY `fk_load_balancer_config__account_id` (`account_id`),
  KEY `idx_load_balancer_config_name` (`name`),
  KEY `idx_load_balancer_config_remove_time` (`remove_time`),
  KEY `idx_load_balancer_config_removed` (`removed`),
  KEY `idx_load_balancer_config_state` (`state`),
  KEY `fk_load_balancer_config__service_id` (`service_id`),
  CONSTRAINT `fk_load_balancer_config__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_config__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer_config`
--

LOCK TABLES `load_balancer_config` WRITE;
/*!40000 ALTER TABLE `load_balancer_config` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer_config` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer_config_listener_map`
--

DROP TABLE IF EXISTS `load_balancer_config_listener_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer_config_listener_map` (
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
  `load_balancer_config_id` bigint(20) DEFAULT NULL,
  `load_balancer_listener_id` bigint(20) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_config_listener_map_uuid` (`uuid`),
  KEY `fk_load_balancer_config_listener_map__load_balancer_config_id` (`load_balancer_config_id`),
  KEY `fk_load_balancer_config_listener_map__load_balancer_listener_id` (`load_balancer_listener_id`),
  KEY `idx_load_balancer_config_listener_map_name` (`name`),
  KEY `idx_load_balancer_config_listener_map_remove_time` (`remove_time`),
  KEY `idx_load_balancer_config_listener_map_removed` (`removed`),
  KEY `idx_load_balancer_config_listener_map_state` (`state`),
  KEY `fk_load_balancer_config_listener_map__account_id` (`account_id`),
  CONSTRAINT `fk_load_balancer_config_listener_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_config_listener_map__load_balancer_config_id` FOREIGN KEY (`load_balancer_config_id`) REFERENCES `load_balancer_config` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_config_listener_map__load_balancer_listener_id` FOREIGN KEY (`load_balancer_listener_id`) REFERENCES `load_balancer_listener` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer_config_listener_map`
--

LOCK TABLES `load_balancer_config_listener_map` WRITE;
/*!40000 ALTER TABLE `load_balancer_config_listener_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer_config_listener_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer_host_map`
--

DROP TABLE IF EXISTS `load_balancer_host_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer_host_map` (
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
  `host_id` bigint(20) DEFAULT NULL,
  `load_balancer_id` bigint(20) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_host_map_uuid` (`uuid`),
  KEY `fk_load_balancer_host_map__host_id` (`host_id`),
  KEY `fk_load_balancer_host_map__load_balancer_id` (`load_balancer_id`),
  KEY `idx_load_balancer_host_map_name` (`name`),
  KEY `idx_load_balancer_host_map_remove_time` (`remove_time`),
  KEY `idx_load_balancer_host_map_removed` (`removed`),
  KEY `idx_load_balancer_host_map_state` (`state`),
  KEY `fk_load_balancer_host_map__account_id` (`account_id`),
  CONSTRAINT `fk_load_balancer_host_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_host_map__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_host_map__load_balancer_id` FOREIGN KEY (`load_balancer_id`) REFERENCES `load_balancer` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer_host_map`
--

LOCK TABLES `load_balancer_host_map` WRITE;
/*!40000 ALTER TABLE `load_balancer_host_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer_host_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer_listener`
--

DROP TABLE IF EXISTS `load_balancer_listener`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer_listener` (
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
  `source_port` int(10) DEFAULT NULL,
  `source_protocol` varchar(255) DEFAULT NULL,
  `target_port` int(10) DEFAULT NULL,
  `target_protocol` varchar(255) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  `private_port` int(11) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_listener_uuid` (`uuid`),
  KEY `fk_load_balancer_listener__account_id` (`account_id`),
  KEY `idx_load_balancer_listener_name` (`name`),
  KEY `idx_load_balancer_listener_remove_time` (`remove_time`),
  KEY `idx_load_balancer_listener_removed` (`removed`),
  KEY `idx_load_balancer_listener_state` (`state`),
  KEY `fk_load_balancer_listener__service_id` (`service_id`),
  CONSTRAINT `fk_load_balancer_listener__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_listener__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer_listener`
--

LOCK TABLES `load_balancer_listener` WRITE;
/*!40000 ALTER TABLE `load_balancer_listener` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer_listener` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `load_balancer_target`
--

DROP TABLE IF EXISTS `load_balancer_target`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `load_balancer_target` (
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
  `ip_address` varchar(255) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `load_balancer_id` bigint(20) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_load_balancer_target_uuid` (`uuid`),
  KEY `fk_load_balancer_target__instance_id` (`instance_id`),
  KEY `fk_load_balancer_target__load_balancer_id` (`load_balancer_id`),
  KEY `idx_load_balancer_target_name` (`name`),
  KEY `idx_load_balancer_target_remove_time` (`remove_time`),
  KEY `idx_load_balancer_target_removed` (`removed`),
  KEY `idx_load_balancer_target_state` (`state`),
  KEY `fk_load_balancer_target__account_id` (`account_id`),
  CONSTRAINT `fk_load_balancer_target__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_target__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_load_balancer_target__load_balancer_id` FOREIGN KEY (`load_balancer_id`) REFERENCES `load_balancer` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `load_balancer_target`
--

LOCK TABLES `load_balancer_target` WRITE;
/*!40000 ALTER TABLE `load_balancer_target` DISABLE KEYS */;
/*!40000 ALTER TABLE `load_balancer_target` ENABLE KEYS */;
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
  `account_id` bigint(20) DEFAULT NULL,
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_machine_driver_uuid` (`uuid`),
  KEY `fk_machine_driver__account_id` (`account_id`),
  KEY `idx_machine_driver_name` (`name`),
  KEY `idx_machine_driver_remove_time` (`remove_time`),
  KEY `idx_machine_driver_removed` (`removed`),
  KEY `idx_machine_driver_state` (`state`),
  CONSTRAINT `fk_machine_driver__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
  `id` bigint(19) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `account_id` bigint(19) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `volume_id` bigint(19) DEFAULT NULL,
  `instance_id` bigint(19) DEFAULT NULL,
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
  `account_id` bigint(20) DEFAULT NULL,
  `kind` varchar(255) NOT NULL,
  `uuid` varchar(128) NOT NULL,
  `description` varchar(1024) DEFAULT NULL,
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  `domain` varchar(128) DEFAULT NULL,
  `network_driver_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_uuid` (`uuid`),
  KEY `fk_network__account_id` (`account_id`),
  KEY `idx_network_name` (`name`),
  KEY `idx_network_remove_time` (`remove_time`),
  KEY `idx_network_removed` (`removed`),
  KEY `idx_network_state` (`state`),
  KEY `fk_network__network_driver_id` (`network_driver_id`),
  CONSTRAINT `fk_network__network_driver_id` FOREIGN KEY (`network_driver_id`) REFERENCES `network_driver` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_driver_uuid` (`uuid`),
  KEY `fk_network_driver__account_id` (`account_id`),
  KEY `fk_network_driver__service_id` (`service_id`),
  KEY `idx_network_driver_name` (`name`),
  KEY `idx_network_driver_remove_time` (`remove_time`),
  KEY `idx_network_driver_removed` (`removed`),
  KEY `idx_network_driver_state` (`state`),
  CONSTRAINT `fk_network_driver__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network_driver__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `network_service`
--

DROP TABLE IF EXISTS `network_service`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `network_service` (
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
  `network_id` bigint(20) DEFAULT NULL,
  `network_service_provider_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_service_uuid` (`uuid`),
  KEY `fk_network_service__account_id` (`account_id`),
  KEY `fk_network_service__network_id` (`network_id`),
  KEY `idx_network_service_name` (`name`),
  KEY `idx_network_service_remove_time` (`remove_time`),
  KEY `idx_network_service_removed` (`removed`),
  KEY `idx_network_service_state` (`state`),
  KEY `fk_network_service__network_service_provider_id` (`network_service_provider_id`),
  CONSTRAINT `fk_network_service__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network_service__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network_service__network_service_provider_id` FOREIGN KEY (`network_service_provider_id`) REFERENCES `network_service_provider` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `network_service`
--

LOCK TABLES `network_service` WRITE;
/*!40000 ALTER TABLE `network_service` DISABLE KEYS */;
/*!40000 ALTER TABLE `network_service` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `network_service_provider`
--

DROP TABLE IF EXISTS `network_service_provider`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `network_service_provider` (
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
  `network_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_service_provider_uuid` (`uuid`),
  KEY `fk_network_service_provider__account_id` (`account_id`),
  KEY `fk_network_service_provider__network_id` (`network_id`),
  KEY `idx_network_service_provider_name` (`name`),
  KEY `idx_network_service_provider_remove_time` (`remove_time`),
  KEY `idx_network_service_provider_removed` (`removed`),
  KEY `idx_network_service_provider_state` (`state`),
  CONSTRAINT `fk_network_service_provider__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_network_service_provider__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `network_service_provider`
--

LOCK TABLES `network_service_provider` WRITE;
/*!40000 ALTER TABLE `network_service_provider` DISABLE KEYS */;
/*!40000 ALTER TABLE `network_service_provider` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `network_service_provider_instance_map`
--

DROP TABLE IF EXISTS `network_service_provider_instance_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `network_service_provider_instance_map` (
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
  `network_service_provider_id` bigint(20) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_network_service_provider_instance_map_uuid` (`uuid`),
  KEY `fk_network_service_provider_instance_map__instance_id` (`instance_id`),
  KEY `fk_nspim_network_service_provider_id` (`network_service_provider_id`),
  KEY `idx_network_service_provider_instance_map_name` (`name`),
  KEY `idx_network_service_provider_instance_map_remove_time` (`remove_time`),
  KEY `idx_network_service_provider_instance_map_removed` (`removed`),
  KEY `idx_network_service_provider_instance_map_state` (`state`),
  CONSTRAINT `fk_network_service_provider_instance_map__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_nspim_network_service_provider_id` FOREIGN KEY (`network_service_provider_id`) REFERENCES `network_service_provider` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `network_service_provider_instance_map`
--

LOCK TABLES `network_service_provider_instance_map` WRITE;
/*!40000 ALTER TABLE `network_service_provider_instance_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `network_service_provider_instance_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `nic`
--

DROP TABLE IF EXISTS `nic`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `nic` (
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
  `instance_id` bigint(20) DEFAULT NULL,
  `network_id` bigint(20) DEFAULT NULL,
  `subnet_id` bigint(20) DEFAULT NULL,
  `vnet_id` bigint(20) DEFAULT NULL,
  `device_number` int(11) DEFAULT NULL,
  `mac_address` varchar(128) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_nic_uuid` (`uuid`),
  KEY `fk_nic__account_id` (`account_id`),
  KEY `fk_nic__instance_id` (`instance_id`),
  KEY `fk_nic__network_id` (`network_id`),
  KEY `fk_nic__subnet_id` (`subnet_id`),
  KEY `fk_nic__vnet_id` (`vnet_id`),
  KEY `idx_nic_name` (`name`),
  KEY `idx_nic_remove_time` (`remove_time`),
  KEY `idx_nic_removed` (`removed`),
  KEY `idx_nic_state` (`state`),
  CONSTRAINT `fk_nic__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_nic__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_nic__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_nic__subnet_id` FOREIGN KEY (`subnet_id`) REFERENCES `subnet` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_nic__vnet_id` FOREIGN KEY (`vnet_id`) REFERENCES `vnet` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `nic`
--

LOCK TABLES `nic` WRITE;
/*!40000 ALTER TABLE `nic` DISABLE KEYS */;
/*!40000 ALTER TABLE `nic` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `offering`
--

DROP TABLE IF EXISTS `offering`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `offering` (
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
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_offering_uuid` (`uuid`),
  KEY `fk_offering__account_id` (`account_id`),
  KEY `idx_offering_name` (`name`),
  KEY `idx_offering_remove_time` (`remove_time`),
  KEY `idx_offering_removed` (`removed`),
  KEY `idx_offering_state` (`state`),
  CONSTRAINT `fk_offering__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `offering`
--

LOCK TABLES `offering` WRITE;
/*!40000 ALTER TABLE `offering` DISABLE KEYS */;
/*!40000 ALTER TABLE `offering` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `physical_host`
--

DROP TABLE IF EXISTS `physical_host`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `physical_host` (
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
  `agent_id` bigint(20) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT '',
  `driver` varchar(128) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_physical_host_uuid` (`uuid`),
  KEY `fk_physical_host__account_id` (`account_id`),
  KEY `idx_physical_host_name` (`name`),
  KEY `idx_physical_host_remove_time` (`remove_time`),
  KEY `idx_physical_host_removed` (`removed`),
  KEY `idx_physical_host_state` (`state`),
  KEY `fk_physical_host__agent_id` (`agent_id`),
  CONSTRAINT `fk_physical_host__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_physical_host__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `physical_host`
--

LOCK TABLES `physical_host` WRITE;
/*!40000 ALTER TABLE `physical_host` DISABLE KEYS */;
/*!40000 ALTER TABLE `physical_host` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `port`
--

DROP TABLE IF EXISTS `port`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `port` (
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
  `public_port` int(11) DEFAULT NULL,
  `private_port` int(11) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `public_ip_address_id` bigint(20) DEFAULT NULL,
  `protocol` varchar(128) NOT NULL,
  `private_ip_address_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_port_uuid` (`uuid`),
  KEY `fk_ip_address__private_ip_address_id` (`private_ip_address_id`),
  KEY `fk_ip_address__public_ip_address_id` (`public_ip_address_id`),
  KEY `fk_port__account_id` (`account_id`),
  KEY `fk_port__instance_id` (`instance_id`),
  KEY `idx_port_name` (`name`),
  KEY `idx_port_remove_time` (`remove_time`),
  KEY `idx_port_removed` (`removed`),
  KEY `idx_port_state` (`state`),
  CONSTRAINT `fk_ip_address__private_ip_address_id` FOREIGN KEY (`private_ip_address_id`) REFERENCES `ip_address` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_ip_address__public_ip_address_id` FOREIGN KEY (`public_ip_address_id`) REFERENCES `ip_address` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_port__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_port__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `port`
--

LOCK TABLES `port` WRITE;
/*!40000 ALTER TABLE `port` DISABLE KEYS */;
/*!40000 ALTER TABLE `port` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  KEY `idx_process_instance_end_time` (`end_time`),
  KEY `idx_process_instance_et_rt_ri` (`end_time`,`resource_type`,`resource_id`),
  KEY `idx_process_instance_priority` (`priority`),
  KEY `idx_process_instance_start_time` (`start_time`),
  KEY `idx_process_instance_run_after` (`run_after`)
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
-- Table structure for table `project_template`
--

DROP TABLE IF EXISTS `project_template`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `project_template` (
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
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  `external_id` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_project_template_uuid` (`uuid`),
  KEY `fk_project_template__account_id` (`account_id`),
  KEY `idx_project_template_name` (`name`),
  KEY `idx_project_template_remove_time` (`remove_time`),
  KEY `idx_project_template_removed` (`removed`),
  KEY `idx_project_template_state` (`state`),
  KEY `idx_project_template_is_public` (`is_public`),
  KEY `idx_project_template_external_id` (`external_id`),
  CONSTRAINT `fk_project_template__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `project_template`
--

LOCK TABLES `project_template` WRITE;
/*!40000 ALTER TABLE `project_template` DISABLE KEYS */;
/*!40000 ALTER TABLE `project_template` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_resource_pool_uuid` (`uuid`),
  UNIQUE KEY `idx_pool_item2` (`pool_type`,`pool_id`,`qualifier`,`item`),
  KEY `fk_resource_pool__account_id` (`account_id`),
  KEY `idx_resource_pool_name` (`name`),
  KEY `idx_resource_pool_remove_time` (`remove_time`),
  KEY `idx_resource_pool_removed` (`removed`),
  KEY `idx_resource_pool_state` (`state`),
  KEY `idx_pool_owner2` (`pool_type`,`pool_id`,`qualifier`,`owner_type`,`owner_id`),
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
  `selector_link` varchar(4096) DEFAULT NULL,
  `selector_container` varchar(4096) DEFAULT NULL,
  `external_id` varchar(255) DEFAULT NULL,
  `health_state` varchar(128) DEFAULT NULL,
  `system` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_uuid` (`uuid`),
  KEY `fk_service__account_id` (`account_id`),
  KEY `fk_service__environment_id` (`environment_id`),
  KEY `idx_service_name` (`name`),
  KEY `idx_service_remove_time` (`remove_time`),
  KEY `idx_service_removed` (`removed`),
  KEY `idx_service_state` (`state`),
  KEY `idx_service_external_id` (`external_id`),
  CONSTRAINT `fk_service__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `environment` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `service_consume_map`
--

DROP TABLE IF EXISTS `service_consume_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `service_consume_map` (
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
  `consumed_service_id` bigint(20) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_consume_map_uuid` (`uuid`),
  KEY `fk_service_consume_map__consumed_service_id` (`consumed_service_id`),
  KEY `fk_service_consume_map__service_id` (`service_id`),
  KEY `idx_service_consume_map_name` (`name`),
  KEY `idx_service_consume_map_remove_time` (`remove_time`),
  KEY `idx_service_consume_map_removed` (`removed`),
  KEY `idx_service_consume_map_state` (`state`),
  KEY `fk_service_consume_map__account_id` (`account_id`),
  CONSTRAINT `fk_service_consume_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_consume_map__consumed_service_id` FOREIGN KEY (`consumed_service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_consume_map__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_consume_map`
--

LOCK TABLES `service_consume_map` WRITE;
/*!40000 ALTER TABLE `service_consume_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `service_consume_map` ENABLE KEYS */;
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
  `healthcheck_instance_id` bigint(20) DEFAULT NULL,
  `reported_health` varchar(255) DEFAULT NULL,
  `external_timestamp` int(11) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_event_uuid` (`uuid`),
  KEY `fk_service_event__account_id` (`account_id`),
  KEY `fk_service_event__healthcheck_instance_id` (`healthcheck_instance_id`),
  KEY `fk_service_event__host_id` (`host_id`),
  KEY `fk_service_event__instance_id` (`instance_id`),
  KEY `idx_service_event_name` (`name`),
  KEY `idx_service_event_remove_time` (`remove_time`),
  KEY `idx_service_event_removed` (`removed`),
  KEY `idx_service_event_state` (`state`),
  CONSTRAINT `fk_service_event__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_event__healthcheck_instance_id` FOREIGN KEY (`healthcheck_instance_id`) REFERENCES `healthcheck_instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
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
-- Table structure for table `service_expose_map`
--

DROP TABLE IF EXISTS `service_expose_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `service_expose_map` (
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
  `instance_id` bigint(20) DEFAULT NULL,
  `account_id` bigint(20) DEFAULT NULL,
  `ip_address` varchar(255) DEFAULT NULL,
  `dns_prefix` varchar(128) DEFAULT NULL,
  `host_name` varchar(255) DEFAULT NULL,
  `managed` bit(1) NOT NULL DEFAULT b'1',
  `upgrade` bit(1) NOT NULL DEFAULT b'0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_instance_map_uuid` (`uuid`),
  KEY `fk_service_instance_map__instance_id` (`instance_id`),
  KEY `fk_service_instance_map__service_id` (`service_id`),
  KEY `idx_service_instance_map_name` (`name`),
  KEY `idx_service_instance_map_remove_time` (`remove_time`),
  KEY `idx_service_instance_map_removed` (`removed`),
  KEY `idx_service_instance_map_state` (`state`),
  KEY `fk_service_expose_map__account_id` (`account_id`),
  CONSTRAINT `fk_service_expose_map__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_instance_map__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_instance_map__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_expose_map`
--

LOCK TABLES `service_expose_map` WRITE;
/*!40000 ALTER TABLE `service_expose_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `service_expose_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `service_index`
--

DROP TABLE IF EXISTS `service_index`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `service_index` (
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
  `launch_config_name` varchar(255) DEFAULT NULL,
  `service_id` bigint(20) DEFAULT NULL,
  `address` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_suffix_uuid` (`uuid`),
  KEY `fk_service_suffix__account_id` (`account_id`),
  KEY `fk_service_suffix__service_id` (`service_id`),
  KEY `idx_service_suffix_name` (`name`),
  KEY `idx_service_suffix_remove_time` (`remove_time`),
  KEY `idx_service_suffix_removed` (`removed`),
  KEY `idx_service_suffix_state` (`state`),
  CONSTRAINT `fk_service_suffix__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_suffix__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `service_index`
--

LOCK TABLES `service_index` WRITE;
/*!40000 ALTER TABLE `service_index` DISABLE KEYS */;
/*!40000 ALTER TABLE `service_index` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  KEY `fk_service_log__account_id` (`account_id`),
  KEY `fk_service_log__instance_id` (`instance_id`),
  KEY `fk_service_log__service_id` (`service_id`),
  CONSTRAINT `fk_service_log__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_log__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_service_log__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `snapshot`
--

DROP TABLE IF EXISTS `snapshot`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `snapshot` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_snapshot_uuid` (`uuid`),
  KEY `fk_snapshot__account_id` (`account_id`),
  KEY `fk_snapshot__volume_id` (`volume_id`),
  KEY `idx_snapshot_name` (`name`),
  KEY `idx_snapshot_remove_time` (`remove_time`),
  KEY `idx_snapshot_removed` (`removed`),
  KEY `idx_snapshot_state` (`state`),
  CONSTRAINT `fk_snapshot__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_snapshot__volume_id` FOREIGN KEY (`volume_id`) REFERENCES `volume` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `snapshot`
--

LOCK TABLES `snapshot` WRITE;
/*!40000 ALTER TABLE `snapshot` DISABLE KEYS */;
/*!40000 ALTER TABLE `snapshot` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `snapshot_storage_pool_map`
--

DROP TABLE IF EXISTS `snapshot_storage_pool_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `snapshot_storage_pool_map` (
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
  `snapshot_id` bigint(20) DEFAULT NULL,
  `storage_pool_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_snapshot_storage_pool_map_uuid` (`uuid`),
  KEY `fk_snapshot_storage_pool_map__snapshot_id` (`snapshot_id`),
  KEY `fk_snapshot_storage_pool_map__storage_pool_id` (`storage_pool_id`),
  KEY `idx_snapshot_storage_pool_map_name` (`name`),
  KEY `idx_snapshot_storage_pool_map_remove_time` (`remove_time`),
  KEY `idx_snapshot_storage_pool_map_removed` (`removed`),
  KEY `idx_snapshot_storage_pool_map_state` (`state`),
  CONSTRAINT `fk_snapshot_storage_pool_map__snapshot_id` FOREIGN KEY (`snapshot_id`) REFERENCES `snapshot` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_snapshot_storage_pool_map__storage_pool_id` FOREIGN KEY (`storage_pool_id`) REFERENCES `storage_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `snapshot_storage_pool_map`
--

LOCK TABLES `snapshot_storage_pool_map` WRITE;
/*!40000 ALTER TABLE `snapshot_storage_pool_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `snapshot_storage_pool_map` ENABLE KEYS */;
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_storage_driver_uuid` (`uuid`),
  KEY `fk_storage_driver__account_id` (`account_id`),
  KEY `fk_storage_driver__service_id` (`service_id`),
  KEY `idx_storage_driver_name` (`name`),
  KEY `idx_storage_driver_remove_time` (`remove_time`),
  KEY `idx_storage_driver_removed` (`removed`),
  KEY `idx_storage_driver_state` (`state`),
  CONSTRAINT `fk_storage_driver__service_id` FOREIGN KEY (`service_id`) REFERENCES `service` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_driver__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
  `account_id` bigint(20) DEFAULT NULL,
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_storage_pool_uuid` (`uuid`),
  KEY `fk_storage_pool__account_id` (`account_id`),
  KEY `fk_storage_pool__agent_id` (`agent_id`),
  KEY `fk_storage_pool__zone_id` (`zone_id`),
  KEY `idx_storage_pool_name` (`name`),
  KEY `idx_storage_pool_remove_time` (`remove_time`),
  KEY `idx_storage_pool_removed` (`removed`),
  KEY `idx_storage_pool_state` (`state`),
  KEY `fk_storage_driver__id` (`storage_driver_id`),
  CONSTRAINT `fk_storage_driver__id` FOREIGN KEY (`storage_driver_id`) REFERENCES `storage_driver` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_pool__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_pool__agent_id` FOREIGN KEY (`agent_id`) REFERENCES `agent` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_storage_pool__zone_id` FOREIGN KEY (`zone_id`) REFERENCES `zone` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
  `account_id` bigint(20) DEFAULT NULL,
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
  `is_public` bit(1) NOT NULL DEFAULT b'0',
  `ip_pool_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_subnet_uuid` (`uuid`),
  KEY `fk_subnet__account_id` (`account_id`),
  KEY `fk_subnet__network_id` (`network_id`),
  KEY `idx_subnet_name` (`name`),
  KEY `idx_subnet_remove_time` (`remove_time`),
  KEY `idx_subnet_removed` (`removed`),
  KEY `idx_subnet_state` (`state`),
  KEY `fk_subnet__pool_id` (`ip_pool_id`),
  CONSTRAINT `fk_subnet__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_subnet__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_subnet__pool_id` FOREIGN KEY (`ip_pool_id`) REFERENCES `ip_pool` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
-- Table structure for table `subnet_vnet_map`
--

DROP TABLE IF EXISTS `subnet_vnet_map`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `subnet_vnet_map` (
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
  `subnet_id` bigint(20) DEFAULT NULL,
  `vnet_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_subnet_vnet_map_uuid` (`uuid`),
  KEY `fk_subnet_vnet_map__subnet_id` (`subnet_id`),
  KEY `fk_subnet_vnet_map__vnet_id` (`vnet_id`),
  KEY `idx_subnet_vnet_map_name` (`name`),
  KEY `idx_subnet_vnet_map_remove_time` (`remove_time`),
  KEY `idx_subnet_vnet_map_removed` (`removed`),
  KEY `idx_subnet_vnet_map_state` (`state`),
  CONSTRAINT `fk_subnet_vnet_map__subnet_id` FOREIGN KEY (`subnet_id`) REFERENCES `subnet` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_subnet_vnet_map__vnet_id` FOREIGN KEY (`vnet_id`) REFERENCES `vnet` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `subnet_vnet_map`
--

LOCK TABLES `subnet_vnet_map` WRITE;
/*!40000 ALTER TABLE `subnet_vnet_map` DISABLE KEYS */;
/*!40000 ALTER TABLE `subnet_vnet_map` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `task`
--

DROP TABLE IF EXISTS `task`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `task` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(128) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_task_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `task`
--

LOCK TABLES `task` WRITE;
/*!40000 ALTER TABLE `task` DISABLE KEYS */;
/*!40000 ALTER TABLE `task` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `task_instance`
--

DROP TABLE IF EXISTS `task_instance`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `task_instance` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(128) NOT NULL,
  `task_id` bigint(20) NOT NULL,
  `start_time` datetime NOT NULL,
  `end_time` datetime DEFAULT NULL,
  `exception` varchar(255) DEFAULT NULL,
  `server_id` varchar(128) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_task_instance__task_id` (`task_id`),
  CONSTRAINT `fk_task_instance__task_id` FOREIGN KEY (`task_id`) REFERENCES `task` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `task_instance`
--

LOCK TABLES `task_instance` WRITE;
/*!40000 ALTER TABLE `task_instance` DISABLE KEYS */;
/*!40000 ALTER TABLE `task_instance` ENABLE KEYS */;
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
  `state` varchar(128) NOT NULL,
  `created` datetime DEFAULT NULL,
  `removed` datetime DEFAULT NULL,
  `remove_time` datetime DEFAULT NULL,
  `data` mediumtext,
  `value` mediumtext NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_preference_uuid` (`uuid`),
  KEY `fk_user_preference__account_id` (`account_id`),
  KEY `idx_user_preference_name` (`name`),
  KEY `idx_user_preference_remove_time` (`remove_time`),
  KEY `idx_user_preference_removed` (`removed`),
  KEY `idx_user_preference_state` (`state`),
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
-- Table structure for table `vnet`
--

DROP TABLE IF EXISTS `vnet`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `vnet` (
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
  `network_id` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_vnet_uuid` (`uuid`),
  KEY `fk_vnet__account_id` (`account_id`),
  KEY `fk_vnet__network_id` (`network_id`),
  KEY `idx_vnet_name` (`name`),
  KEY `idx_vnet_remove_time` (`remove_time`),
  KEY `idx_vnet_removed` (`removed`),
  KEY `idx_vnet_state` (`state`),
  CONSTRAINT `fk_vnet__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_vnet__network_id` FOREIGN KEY (`network_id`) REFERENCES `network` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `vnet`
--

LOCK TABLES `vnet` WRITE;
/*!40000 ALTER TABLE `vnet` DISABLE KEYS */;
/*!40000 ALTER TABLE `vnet` ENABLE KEYS */;
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
  `device_number` int(11) DEFAULT NULL,
  `format` varchar(255) DEFAULT NULL,
  `allocation_state` varchar(255) DEFAULT NULL,
  `attached_state` varchar(255) DEFAULT NULL,
  `instance_id` bigint(20) DEFAULT NULL,
  `image_id` bigint(20) DEFAULT NULL,
  `offering_id` bigint(20) DEFAULT NULL,
  `zone_id` bigint(20) DEFAULT NULL,
  `uri` varchar(255) DEFAULT NULL,
  `external_id` varchar(128) DEFAULT NULL,
  `access_mode` varchar(255) DEFAULT NULL,
  `host_id` bigint(20) DEFAULT NULL,
  `deployment_unit_id` bigint(20) DEFAULT NULL,
  `environment_id` bigint(20) DEFAULT NULL,
  `volume_template_id` bigint(20) DEFAULT NULL,
  `storage_driver_id` bigint(20) DEFAULT NULL,
  `size_mb` bigint(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_volume_uuid` (`uuid`),
  KEY `fk_volume__account_id` (`account_id`),
  KEY `fk_volume__image_id` (`image_id`),
  KEY `fk_volume__instance_id` (`instance_id`),
  KEY `fk_volume__offering_id` (`offering_id`),
  KEY `fk_volume__zone_id` (`zone_id`),
  KEY `idx_volume_name` (`name`),
  KEY `idx_volume_remove_time` (`remove_time`),
  KEY `idx_volume_removed` (`removed`),
  KEY `idx_volume_state` (`state`),
  KEY `idx_volume_external_id` (`external_id`),
  KEY `idx_volume_uri` (`uri`(255)),
  KEY `fk_volume__host_id` (`host_id`),
  KEY `fk_volume__deployment_unit_id` (`deployment_unit_id`),
  KEY `fk_volume__environment_id` (`environment_id`),
  KEY `fk_volume__volume_template_id` (`volume_template_id`),
  KEY `fk_volume__storage_driver_id` (`storage_driver_id`),
  CONSTRAINT `fk_volume__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__deployment_unit_id` FOREIGN KEY (`deployment_unit_id`) REFERENCES `deployment_unit` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `environment` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__host_id` FOREIGN KEY (`host_id`) REFERENCES `host` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__image_id` FOREIGN KEY (`image_id`) REFERENCES `image` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__instance_id` FOREIGN KEY (`instance_id`) REFERENCES `instance` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__offering_id` FOREIGN KEY (`offering_id`) REFERENCES `offering` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__storage_driver_id` FOREIGN KEY (`storage_driver_id`) REFERENCES `storage_driver` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__volume_template_id` FOREIGN KEY (`volume_template_id`) REFERENCES `volume_template` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume__zone_id` FOREIGN KEY (`zone_id`) REFERENCES `zone` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_volume_template_uuid` (`uuid`),
  KEY `fk_volume_template__account_id` (`account_id`),
  KEY `fk_volume_template__environment_id` (`environment_id`),
  KEY `idx_volume_template_name` (`name`),
  KEY `idx_volume_template_remove_time` (`remove_time`),
  KEY `idx_volume_template_removed` (`removed`),
  KEY `idx_volume_template_state` (`state`),
  CONSTRAINT `fk_volume_template__environment_id` FOREIGN KEY (`environment_id`) REFERENCES `environment` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION,
  CONSTRAINT `fk_volume_template__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `volume_template`
--

LOCK TABLES `volume_template` WRITE;
/*!40000 ALTER TABLE `volume_template` DISABLE KEYS */;
/*!40000 ALTER TABLE `volume_template` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `zone`
--

DROP TABLE IF EXISTS `zone`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `zone` (
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
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_zone_uuid` (`uuid`),
  KEY `fk_zone__account_id` (`account_id`),
  KEY `idx_zone_name` (`name`),
  KEY `idx_zone_remove_time` (`remove_time`),
  KEY `idx_zone_removed` (`removed`),
  KEY `idx_zone_state` (`state`),
  CONSTRAINT `fk_zone__account_id` FOREIGN KEY (`account_id`) REFERENCES `account` (`id`) ON DELETE NO ACTION ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `zone`
--

LOCK TABLES `zone` WRITE;
/*!40000 ALTER TABLE `zone` DISABLE KEYS */;
INSERT INTO `zone` VALUES (1,'zone1',NULL,'zone','zone1',NULL,'active',NULL,NULL,NULL,NULL);
/*!40000 ALTER TABLE `zone` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2016-11-29 21:10:25
