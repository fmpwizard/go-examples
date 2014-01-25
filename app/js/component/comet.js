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
      console.log('sessionId ' + sessionId);
      setTimeout(function(){
        $.ajax({ url: '/api/comet/' + pageId + '/' + sessionId , success: function(data){
          console.log(data);
          self.trigger('start-long-pool', {
            delay: 0,
            pageId: pageId,
            sessionId: sessionId
          });
          $(document).trigger(data.event, {
            message: data.data
          });
        },
        dataType: 'json',
        timeout: 30000 ,
        error: function(){
          self.trigger('start-long-pool', {
            delay: delay + 1000,
            pageId: pageId,
            sessionId: sessionId
          });
        }
        });
      },delay);
    };

    this.after('initialize', function () {
      this.on('start-long-pool', this.startLongPool);
      this.trigger('start-long-pool', {
        delay: 0,
        pageId: Math.random().toString(36).substring(7),
        sessionId: Math.random().toString(36).substring(7)
      });
    });
  }

});
