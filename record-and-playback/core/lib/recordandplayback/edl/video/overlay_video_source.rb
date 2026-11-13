# frozen_string_literal: true

module BigBlueButton
  module EDL
    module Video
      # EDL Video Source for the context overlay
      #
      # The overlay is a full frame picture with an alpha channel which is composited on top of everything else. It
      # carries the meeting name, date and elapsed time, who is speaking, the name of each webcam and a timeline with
      # the session activity (chat, polls, external videos, ...).
      #
      # Unlike the other video sources it is not placed in a layout area: it always covers the whole frame, and it has
      # to be told where the media was placed in each cut, so it can leave those regions transparent.
      #
      # @param raw_dir [String] directory containing the raw recording data (events.xml file)
      # @param process_dir [String] directory for output files from processing
      # @param config [Hash] the 'overlay' section of the preset
      class OverlayVideoSource
        def initialize(raw_dir, process_dir, config)
          @raw_dir = raw_dir
          @process_dir = process_dir
          @config = config.transform_keys(&:to_s).reject { |key, _value| key == 'enabled' }
        end

        def aspect_ratio
          nil
        end

        def corrupt?
          false
        end

        def pts_gaps
          []
        end

        # Generate a cut of the overlay video using the bbb-recording-overlay tool
        #
        # @param width [Integer] width of the output video
        # @param height [Integer] height of the output video
        # @param seek [Integer] start time of the cut in milliseconds
        # @param duration [Integer] duration of the cut in milliseconds
        # @param framerate [Float] frame rate of the output video
        # @param name [String] name which uniquely identifies the individual cut of this video.
        # @param areas [Array<Hash>] position and size of the media areas visible in this cut
        # @param webcams [Array<Hash>] position, size and user of each webcam tile in this cut
        # @param hidden_webcams [Integer] number of webcams left out of this cut (see Events.edl_limit_webcams)
        def open(width, height, seek, duration, framerate, name, areas: [], webcams: [], hidden_webcams: 0)
          read, write = IO.pipe
          begin
            write.fcntl(Fcntl::F_SETPIPE_SZ, 1_048_576)
          rescue Errno::EPERM
            BigBlueButton.logger.warn('Unable to increase pipe size to 1MB')
          rescue NameError
            # Fcntl::F_SETPIPE_SZ isn't available on Ruby version older than 3.0
          end

          config = @config.merge(
            'areas' => areas, 'webcams' => webcams, 'hidden_webcams' => hidden_webcams,
            # Downloaded pictures of the users are kept here, shared by all the cuts
            'cache_dir' => File.join(@process_dir, 'overlay-cache')
          )
          command = [
            'bbb-recording-overlay',
            '-w', width.to_s,
            '-h', height.to_s,
            '-r', framerate.to_s,
            '-i', @raw_dir,
            '-o', "/dev/fd/#{write.fileno}",
            '-s', BigBlueButton::EDL::Video.ms_to_s(seek),
            '-e', BigBlueButton::EDL::Video.ms_to_s(seek + duration),
            '--config', JSON.generate(config),
          ]
          log_file = File.join(@process_dir, "bbb-recording-overlay-#{name}.log")

          BigBlueButton.logger.debug("Executing: #{Shellwords.join(command)}")
          pid = spawn(
            *command,
            close_others: true,
            out: log_file,
            err: %i[child out],
            write => write
          )
          write.close
          BigBlueButton.logger.debug("bbb-recording-overlay pid #{pid}")

          ffmpeg_input = [
            '-f', 'rawvideo',
            '-pixel_format', 'bgra', # NOTE: bgra for little-endian; argb for big-endian
            '-video_size', "#{width}x#{height}",
            '-framerate', framerate.to_s,
            '-i', "pipe:#{read.fileno}",
          ]

          VideoSourceReader.new(
            pid: pid,
            read: read,
            ffmpeg_input: ffmpeg_input,
            log_file: log_file
          )
        end
      end
    end
  end
end
