import http from 'k6/http';
import crypto from 'k6/crypto';
import { check, sleep } from "k6";

// Webhook secret
const secret = 'your_secret_here';

// Helper function to generate signature
function generateSignature(payload) {
  const hmac = crypto.createHMAC('sha256', secret);
  hmac.update(payload);
  return `sha256=${hmac.digest('hex')}`;
}

// Test configuration
export const options = {
    thresholds: {
      http_req_duration: ["p(99) < 300"],
    },
    stages: [
      { duration: "30s", target: 15 },
      { duration: "200s", target: 15 },
      { duration: "20s", target: 0 },
    ],
};

// Webhook URL
const webhookUrl = 'http://localhost:8080/webhook'; // Adjust as needed

// Shared Map to track running jobs across VUs
const runningJobs = new Map();

// Create a new webhook event payload
function createWebhookEvent(jobId, status, baseTime) {
  const labels = Math.random() < 0.5 ? ['self-hosted'] : [];
  const now = new Date();
  const payload = {
    action: status,
    workflow_job: {
      id: jobId,
      labels: labels,
      created_at: baseTime.toISOString(),
    }
  };
  
  if (status === 'in_progress') {
    payload.workflow_job.started_at = now.toISOString();
  } else if (status === 'completed') {
    payload.workflow_job.started_at = runningJobs.get(jobId).startedAt;
    payload.workflow_job.completed_at = now.toISOString();
  }
  
  return payload;
}

// Send a webhook event
function sendWebhookEvent(jobId, status, baseTime) {
  const payload = createWebhookEvent(jobId, status, baseTime);
  const payloadStr = `payload=${encodeURIComponent(JSON.stringify(payload))}`;
  const signature = generateSignature(payloadStr);

  const res = http.post(webhookUrl, payloadStr, {
    headers: { 
      'Content-Type': 'application/x-www-form-urlencoded',
      'X-Hub-Signature-256': signature
    },
  });

  check(res, { 
    "status was 200": (r) => r.status == 200,
    "response has success status": (r) => r.json().status === "success"
  });
}

// Default function (main test scenario)
export default function() {
  const jobId = (parseInt(__VU) * 10000) + parseInt(__ITER);
  const baseTime = new Date();

  // First, queue the job
  sendWebhookEvent(jobId, 'queued', baseTime);
  runningJobs.set(jobId, { status: 'queued', createdAt: baseTime });
  sleep(Math.random() * 2 + 1); // Random sleep 1-3 seconds

  // Then move it to in_progress
  const startedAt = new Date();
  sendWebhookEvent(jobId, 'in_progress', baseTime);
  runningJobs.set(jobId, { status: 'in_progress', startedAt: startedAt });
  sleep(Math.random() * 3 + 2); // Random sleep 2-5 seconds

  // Finally complete the job
  sendWebhookEvent(jobId, 'completed', baseTime);
  runningJobs.delete(jobId);
  
  // Add some variability between job iterations
  sleep(Math.random() * 2);
}