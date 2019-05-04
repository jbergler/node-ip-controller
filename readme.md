node-ip-controller is a Kubernetes Controller that watches a cluster 
to keep a DNS record updated with the external IP's of all healthy 
nodes.

I use this to run a small cluster in GKE for my personal projects
without needing to pay for a load balancer. With a single pre-emptible
g1-small instance the monthly cost is around $5.

To make this useful, I've exposed the nginx-ingress using HostPorts.

# Setup
First, make sure your gcloud and kubectl cli's are correctly configured.

We're going to run this service in it's own namespace, so let's create
that first.

```
kubectl create namespace node-ip-controller
```

Then we're going to creating a GCP service account with permissions to
make DNS changes. 
```
PROJECT_ID=$(gcloud config get-value project)

# Create a service account
gcloud iam service-accounts \
  create node-ip-service-account --display-name "NodeIP DNS"

# Grant it the dns.admin role
gcloud projects add-iam-policy-binding \
  $PROJECT_ID \
  --member serviceAccount:node-ip-service-account@$PROJECT_ID.iam.gserviceaccount.com \
  --role /roles/dns.admin

# Create an key and save it to disk so we can add it to k8s.
gcloud iam service-accounts keys create \
  key.json \
  --iam-account node-ip-service-account@$PROJECT_ID.iam.gserviceaccount.com

# Add the secret to kubernetes
kubectl create secret generic \
  node-ip-dns-credentials \
  --from-file key.json \
  --namespace node-ip-controller

# And lastly remove the secret from disk again
rm key.json
```

And now we're ready to deploy things to kubernetes.
```
kubectl apply -f deploy.yml
```

Lastly, we need to ensure the configuration is correct.
```
# edit config.yml, then
kubectl apply -f config.yml
```

