using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class MapMover : MonoBehaviour
{
    public GameObject prefab, back, middle, front;
    public float prefabSizeZ, cameraNotSeePos;    

    private void Update()
    {
        if(front.transform.position.z > cameraNotSeePos)
        {
            Destroy(front);
            front = middle;
            middle = back;
            back = Instantiate(prefab, back.transform.position - new Vector3(0, 0, prefabSizeZ), Quaternion.identity);
        }
    }
}
