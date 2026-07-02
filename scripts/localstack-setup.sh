#!/bin/bash
echo "Inicializando LocalStack SQS..."

awslocal sqs create-queue --queue-name incidentes-queue

echo "Cola SQS 'incidentes-queue' creada."
