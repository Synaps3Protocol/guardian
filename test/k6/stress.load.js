import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate } from "k6/metrics";

const endpoint = "https://g.watchit.movie"
// A custom metric to track failure rates
const failureRate = new Rate("check_failure_rate");
// sample endpoints
const content = `${endpoint}/content/bafkreiaovz6mh6ejmndthqrzpcboy2shzxfi6lghlcysbdnbm35fzlyhyu/`
const metadata = `${endpoint}/metadata/f01551220b56b4833e0d9329c7d537638d2637edb708fef507f38b4655e29809d87e1995b/`

// Options
export let options = {
    stages: [
        // Linearly ramp up from 1 to 150 VUs during first minute
        { target: 200, duration: "3m" },
        // Hold at 100 VUs for the next 3 minutes and 30 seconds
        { target: 150, duration: "8m30s" },
        // Linearly ramp down from 50 to 30 VUs over the last 1 minute
        { target: 30, duration: "1m" },
        // Linearly ramp down from 50 to 0 VUs over the last 30 seconds
        { target: 0, duration: "30s" }
    ],
    thresholds: {
        'group_duration{group:::Metadata}': ['avg < 200'],
        'group_duration{group:::Content}': ['avg < 1000'],
        'http_req_failed': ['rate<0.01'], // http errors should be less than 1%
        // We want the 95th percentile of all HTTP request durations to be less than 500ms
        "http_req_duration{group:::Metadata}": ["p(95)<200"],
        "http_req_duration{group:::Content}": ["p(95)<1000"],
        // Thresholds based on the custom metric we defined and use to track application failures
        "check_failure_rate": [
            // Global failure rate should be less than 1%
            "rate<0.01",
            // Abort the test early if it climbs over 5%
            { threshold: "rate<=0.05", abortOnFail: true },
        ],
    },
};

// Main function
export default function () {


    group("Metadata", function () {
        // Execute multiple requests in parallel like a browser, to fetch some static resources
        let resps = http.batch([
            ["GET", metadata]
        ]);

        // Combine check() call with failure tracking
        failureRate.add(!check(resps, {
            // Expected status for block endpoints
            "status is 200": (r) => r[0].status === 200,
            "verify valid response": (r) => r[0].json("Type") == "application/vnd.apple.mpegurl"
        }));
    });


    group("Content", function () {
        // Execute multiple requests in parallel like a browser, to fetch some static resources
        let resps = http.batch([
            ["GET", content]
        ]);

        // Combine check() call with failure tracking
        failureRate.add(!check(resps, {
            // Expected status for transactions endpoints
            "status is 200": (r) => r[0].status === 200,
        }));
    });


    sleep(Math.random() * 3); // Random sleep between 2s and 5s
}