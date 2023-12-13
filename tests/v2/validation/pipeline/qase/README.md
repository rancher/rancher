# Qase Reporting

The package contains two main.go files, that use the qase-go client to perform API calls to Qase. Reporter updates Qase with test cases and their statuses, and testrun starts and ends test runs for our recurring runs pipelines.

## Table of Contents
1. [Reporter](#Reporter)
2. [Test Run](#Test-Run)

## Reporter
Reporter retreives all test cases inorder to determine if said automation test exists or not. If it does not it will create the test case. There is a custom field for automation test name, so we can update results for existing tests. This is to determine if a pre-existing manual test case has been automated. This value should be the package and test name, ex: TestTokenTestSuite/TestPatchTokenTest1. It will then update the status of the test case, for a specifc test run provided. 

## Test Run
Test run is primarily used to create a test run for our different recurring run pipelines ie daily, weekly and biweekly. There is a custom field in test run for source, so we can filter by how the test run is created.