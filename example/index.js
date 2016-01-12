const goLambda = require(/*'go-lambda*/ '../lambda');

goLambda.init();

exports.handler = function (event, context) {
  goLambda.handle(event, context);
};
