window.onload = function() {
  //<editor-fold desc="Changeable Configuration Block">

  // the following lines will be replaced by docker/configurator, when it runs in a docker-container
  window.ui = SwaggerUIBundle({
    urls: [{"url":"zipkin.swagger.json","name":"zipkin.swagger.json"},{"url":"storage/v2/trace_storage.swagger.json","name":"storage/v2/trace_storage.swagger.json"},{"url":"storage/v2/dependency_storage.swagger.json","name":"storage/v2/dependency_storage.swagger.json"},{"url":"storage/v2/attributes_storage.swagger.json","name":"storage/v2/attributes_storage.swagger.json"},{"url":"api_v2/query.swagger.json","name":"api_v2/query.swagger.json"},{"url":"api_v2/sampling.swagger.json","name":"api_v2/sampling.swagger.json"},{"url":"api_v2/model.swagger.json","name":"api_v2/model.swagger.json"},{"url":"api_v2/collector.swagger.json","name":"api_v2/collector.swagger.json"},{"url":"api_v3/query_service.swagger.json","name":"api_v3/query_service.swagger.json"}],
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout"
  });

  //</editor-fold>
};
