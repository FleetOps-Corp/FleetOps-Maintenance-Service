#!/bin/bash
echo "Inicializando LocalStack SQS..."

awslocal sqs create-queue --queue-name incidentes-queue
awslocal sqs create-queue --queue-name vehiculos-queue

echo "Colas SQS 'incidentes-queue' y 'vehiculos-queue' creadas."
