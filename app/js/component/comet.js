define(function (require) {

  'use strict';

  /**
   * Module dependencies
   */

  var defineComponent = require('flight/lib/component');

  /**
   * Module exports
   */

  return defineComponent(comet);

  /**
   * Module function
   */

  function comet() {
    this.defaultAttrs({

    });

    this.startLongPool = function (_, payload) {
      var self = this;
      var delay = payload.delay;
      var pageId = payload.pageId;
      var sessionId = payload.sessionId;
      var index = payload.index;
      var cometId = payload.cometId;
      console.log('sessionId ' + sessionId);
      console.log('pageId ' + pageId);
      console.log('window.cometId ' + window.cometId);
      setTimeout(function(){
        $.ajax({
          url: '/api/comet?sessionid=' + sessionId + '&page=' + pageId + '&index=' + index + '&cometid=' + cometId,
          success: function(data){
            console.log('data', data);
            self.trigger('start-long-pool', {
              delay: 0,
              sessionId: sessionId,
              pageId: pageId,
              index: data.lastIndex,
              cometId: cometId
            });
            $(document).trigger(data.event, {
              message: data
            });
          },
          dataType: 'json',
          timeout: 120000 ,
          error: function(){
            self.trigger('start-long-pool', {
              delay: delay + 1000,
              sessionId: sessionId,
              pageId: pageId,
              index: index,
              cometId: cometId
            });
          }
        });
      },delay);
    };

    this.after('initialize', function () {
      this.on('start-long-pool', this.startLongPool);
      this.trigger('start-long-pool', {
        delay: 0,
        sessionId: Math.random().toString(36).substring(7),
        pageId: Math.random().toString(36).substring(7),
        index: window.index,
        cometId: window.cometId
      });
    });
  }

});
