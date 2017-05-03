#include <stdint.h>
#include <stdbool.h>

///
/// C respresentation of a pst instance It also contains and owns the data that was returned at the
/// last request.
///
/// After initializing the pst by the C interface, a pointer DataStructure object will be returned
/// caller. The pointer should not be modified from outside!
///
/// To get data, the struct pointer must be given to the corresponding function as an argument.
///
typedef void * Datastructure;


///
/// A C representation of a label and its data.
///
/// The result of requests of the data structure will be returned as an c-array of these structs.
///
typedef struct C_Label {
	double x;
	double y;
	double t;
	int64_t osm_id;
	int32_t prio;
	double lbl_fac;
	char* label;
} C_Label;

///
/// A struct represents a basic C_Label vector, i.e. its size and the data (the contained C_Label
/// objects).
///
typedef struct C_Result {
	uint64_t size;
	C_Label* data;
} C_Result;

///
/// Initialize a 3D PST from the file defined by input_path.
///
/// The returned pointer to the DataStructure object can be used to request data from the 3D PST.
///
/// The given file must match the format specified in the [Input Module](input/index.html).
///
Datastructure *init(char const* input_path);

///
/// Check if the initialization was successfull and the returned DataStructure object is valid.
///
bool is_good(Datastructure *ds);

///
/// Get the labels contained in the specified bounding box with a t value >= min_t.
///
C_Result get_data(Datastructure *ds, double min_t, double min_x, double max_x, double min_y, double max_y);


