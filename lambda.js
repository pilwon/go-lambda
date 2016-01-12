const childProcess = require('child_process');
const os = require('os');

const split = require('split');

const MAX_FAILS = 4;
const OS_PLATFORM = os.platform();

var currentRequestId = null;
var dones = {};
var failCount = 0;
var go = null;
var spawn = defaultSpawn;

function callDone(requestId, err, data) {
  var done = requestId ? dones[requestId] : null;
  if (!done) {
    if (err) {
      console.error(err.message);
    }
    console.error(dones);
    console.error('cannot call done for request ID ' + requestId);
    process.exit(1);
  }
  done(err, data);
  delete dones[requestId];
}

function defaultSpawn() {
  if (process.env.NODE_ENV === 'production') {
    return childProcess.spawn('bin/' + OS_PLATFORM + '64', {stdio: ['pipe', 'pipe', process.stderr]});
  } else {
    return childProcess.spawn('go', ['run', 'main.go'], {stdio: ['pipe', 'pipe', process.stderr]});
  }
}

function handleFail() {
  if (++failCount > MAX_FAILS) {
    process.exit(1);  // force container restart
  }
  spawnSubProcess();
}

function spawnSubProcess() {
  go = spawn();
  go.on('error', function (err) {
    process.stderr.write('Go process errored: ' + JSON.stringify(err) + '\n');
    handleFail();
    callDone(currentRequestId, err);
  });
  go.on('exit', function (code) {
    process.stderr.write('Go process exited prematurely with code: ' + code + '\n');
    handleFail();
    callDone(currentRequestId, new Error('Exited with code ' + code));
  });
  go.stdin.on('error', function (err) {
    process.stderr.write('Go process stdin write error: ' + JSON.stringify(err) + '\n');
    handleFail();
    callDone(currentRequestId, err);
  });
  go.stdout.pipe(split()).on('data', function (line) {
    failCount = 0;
    var res = JSON.parse(line.toString('utf-8'));
    if (res.error) {
      callDone(res.id, new Error(res.error));
    } else {
      callDone(res.id, null, res.reply);
    }
  });
}

exports.init = function (spawnOverride) {
  if (go) {
    console.error('go-lambda already initialized');
    process.exit(1);
  } else if (typeof spawnOverride != 'undefined') {
    if (typeof spawnOverride != 'function') {
      console.error('spawnOverride must be function');
      process.exit(1);
    }
    spawn = spawnOverride;
  }
  spawnSubProcess();
};

exports.handle = function (event, context) {
  if (!go) {
    console.error('go-lambda not initialized');
    process.exit(1);
  }
  currentRequestId = context.awsRequestId;
  dones[currentRequestId] = context.done.bind(context);
  go.stdin.write(JSON.stringify({context: context, event: event}) + '\n');
};
